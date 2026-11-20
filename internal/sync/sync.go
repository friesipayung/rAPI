// Package sync implements debounced auto-commit + auto-push on top of the
// git engine, with a fsnotify watcher and a sync-status event feed.
package sync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/model"
)

const (
	commitDebounce = 800 * time.Millisecond
	pushDebounce   = 3 * time.Second
	logCapacity    = 50
)

// ErrNoVault is returned when operating without an active vault.
var ErrNoVault = errors.New("no vault open")

// Syncer watches the vault and keeps it committed and pushed.
type Syncer struct {
	g git.Gitter

	mu      sync.Mutex
	dir     string
	watcher *fsnotify.Watcher
	emit    func(model.SyncStatus)
	log     []model.GitLogEntry
	ctx     context.Context
	cancel  context.CancelFunc

	commitTimer *time.Timer
	pushTimer   *time.Timer
	state       string
}

// New creates a Syncer using the given git engine.
func New(g git.Gitter) *Syncer {
	return &Syncer{g: g, state: model.SyncIdle}
}

// Start watches dir and emits status changes through emit.
func (s *Syncer) Start(dir string, emit func(model.SyncStatus)) error {
	s.Stop()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.dir = dir
	s.watcher = w
	s.ctx = ctx
	s.cancel = cancel
	s.emit = emit
	s.state = model.SyncIdle
	s.mu.Unlock()

	// Watch recursively (skip .git).
	if err := s.watchRecursive(dir); err != nil {
		w.Close()
		return err
	}
	go s.loop()
	s.emitStatus(model.SyncStatus{State: model.SyncIdle, Detail: "watching " + filepath.Base(dir), At: now()})
	return nil
}

// Stop tears the watcher down and cancels pending timers.
func (s *Syncer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}
	if s.commitTimer != nil {
		s.commitTimer.Stop()
	}
	if s.pushTimer != nil {
		s.pushTimer.Stop()
	}
	s.dir = ""
	s.state = model.SyncIdle
}

func (s *Syncer) watchRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}
		s.mu.Lock()
		w := s.watcher
		s.mu.Unlock()
		if w == nil {
			return filepath.SkipAll
		}
		_ = w.Add(path)
		return nil
	})
}

// loop consumes watcher events; mutations schedule the debounced commit.
func (s *Syncer) loop() {
	for {
		s.mu.Lock()
		w, ctx := s.watcher, s.ctx
		s.mu.Unlock()
		if w == nil || ctx == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			s.handleEvent(ev)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

func (s *Syncer) handleEvent(ev fsnotify.Event) {
	// New directories need their own watcher; .git is never watched.
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() && filepath.Base(ev.Name) != ".git" {
			s.mu.Lock()
			w := s.watcher
			s.mu.Unlock()
			if w != nil {
				_ = w.Add(ev.Name)
			}
		}
	}
	if ev.Has(fsnotify.Chmod) {
		return
	}
	s.scheduleCommit()
}

func (s *Syncer) scheduleCommit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return
	}
	s.setStateLocked(model.SyncPending, "changes detected")
	if s.commitTimer != nil {
		s.commitTimer.Reset(commitDebounce)
		return
	}
	s.commitTimer = time.AfterFunc(commitDebounce, func() {
		s.mu.Lock()
		s.commitTimer = nil
		s.mu.Unlock()
		s.doCommit("auto: vault changes")
	})
}

// doCommit runs commit + (on success) schedules the debounced push.
func (s *Syncer) doCommit(msg string) {
	dir := s.currentDir()
	if dir == "" {
		return
	}
	ctx := s.currentCtx()
	s.emitStatus(model.SyncStatus{State: model.SyncSyncing, Detail: "committing", At: now()})

	committed, err := git.Commit(ctx, s.g, dir, msg)
	if err != nil {
		s.fail("commit", err)
		return
	}
	if !committed {
		s.emitStatus(model.SyncStatus{State: model.SyncSynced, Detail: "nothing to commit", At: now()})
		return
	}
	s.schedulePush()
}

func (s *Syncer) schedulePush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pushTimer != nil {
		s.pushTimer.Reset(pushDebounce)
		return
	}
	s.pushTimer = time.AfterFunc(pushDebounce, func() {
		s.mu.Lock()
		s.pushTimer = nil
		s.mu.Unlock()
		s.doPush()
	})
}

func (s *Syncer) doPush() {
	dir := s.currentDir()
	if dir == "" {
		return
	}
	if url := git.RemoteURL(s.currentCtx(), s.g, dir, "origin"); url == "" {
		s.emitStatus(model.SyncStatus{State: model.SyncSynced, Detail: "committed (no remote)", At: now()})
		return
	}
	s.emitStatus(model.SyncStatus{State: model.SyncSyncing, Detail: "pushing", At: now()})
	ctx := s.currentCtx()
	if git.RemoteHasCommits(ctx, s.g, dir) {
		if err := git.PullRebase(ctx, s.g, dir); err != nil {
			s.fail("pull --rebase", err)
			return
		}
	}
	if err := git.Push(ctx, s.g, dir); err != nil {
		s.fail("push", err)
		return
	}
	s.emitStatus(model.SyncStatus{State: model.SyncSynced, Detail: "pushed", At: now()})
}

// CommitNow commits immediately (manual button).
func (s *Syncer) CommitNow(msg string) error {
	if msg == "" {
		msg = "manual: vault changes"
	}
	dir := s.currentDir()
	if dir == "" {
		return ErrNoVault
	}
	// Cancel pending automatic run for this pass.
	s.mu.Lock()
	if s.commitTimer != nil {
		s.commitTimer.Stop()
		s.commitTimer = nil
	}
	s.mu.Unlock()
	s.emitStatus(model.SyncStatus{State: model.SyncSyncing, Detail: "committing", At: now()})
	committed, err := git.Commit(s.currentCtx(), s.g, dir, msg)
	if err != nil {
		s.fail("commit", err)
		return err
	}
	if !committed {
		s.emitStatus(model.SyncStatus{State: model.SyncSynced, Detail: "nothing to commit", At: now()})
		return nil
	}
	s.schedulePush()
	return nil
}

// PushNow pushes immediately (manual button).
func (s *Syncer) PushNow() error {
	dir := s.currentDir()
	if dir == "" {
		return ErrNoVault
	}
	s.doPush()
	return nil
}

// PullNow rebases on top of the remote (manual button, also the documented
// way back from a conflict).
func (s *Syncer) PullNow() error {
	dir := s.currentDir()
	if dir == "" {
		return ErrNoVault
	}
	s.emitStatus(model.SyncStatus{State: model.SyncSyncing, Detail: "pulling", At: now()})
	ctx := s.currentCtx()
	if err := git.PullRebase(ctx, s.g, dir); err != nil {
		s.fail("pull --rebase", err)
		return err
	}
	s.emitStatus(model.SyncStatus{State: model.SyncSynced, Detail: "pulled", At: now()})
	return nil
}

// Status returns the last emitted state.
func (s *Syncer) Status() model.SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.SyncStatus{State: s.state, At: now()}
}

// History returns the ring buffer of git operations (newest first).
func (s *Syncer) History() []model.GitLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.GitLogEntry, len(s.log))
	copy(out, s.log)
	return out
}

// fail records the failure and flips to error/conflict.
func (s *Syncer) fail(op string, err error) {
	stderr := err.Error()
	state := model.SyncError
	if git.IsConflict(stderr) || op == "pull --rebase" {
		state = model.SyncConflict
	}
	s.mu.Lock()
	s.log = append([]model.GitLogEntry{{Time: now(), Op: op, Stderr: stderr, Err: stderr}}, s.log...)
	if len(s.log) > logCapacity {
		s.log = s.log[:logCapacity]
	}
	s.mu.Unlock()
	s.emitStatus(model.SyncStatus{State: state, Detail: stderr, At: now()})
}

func (s *Syncer) setStateLocked(state, detail string) {
	s.state = state
	s.emit(model.SyncStatus{State: state, Detail: detail, At: now()})
}

func (s *Syncer) emitStatus(st model.SyncStatus) {
	s.mu.Lock()
	s.state = st.State
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		emit(st)
	}
}

func (s *Syncer) currentDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}

func (s *Syncer) currentCtx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func now() time.Time { return time.Now().UTC() }
