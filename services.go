package main

import (
	"context"
	"errors"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
	"github.com/friesipayung/rapi/internal/sync"
	"github.com/friesipayung/rapi/internal/vault"
)

// ---------- VaultService ----------

// VaultService exposes vault lifecycle + remotes to the frontend.
type VaultService struct {
	mgr *vault.Manager
	g   git.Gitter
	sy  *sync.Syncer
}

func NewVaultService(mgr *vault.Manager, g git.Gitter, sy *sync.Syncer) *VaultService {
	return &VaultService{mgr: mgr, g: g, sy: sy}
}

// Init scaffolds or adopts a vault (wizard).
func (v *VaultService) Init(ctx context.Context, path, name, remote string) (*model.VaultStatus, error) {
	st, err := v.mgr.Init(ctx, v.g, path, name, remote)
	if err != nil {
		return nil, err
	}
	if err := v.sy.Start(st.Path, SyncEmitter); err != nil {
		return nil, err
	}
	return st, nil
}

// Open activates an existing vault.
func (v *VaultService) Open(ctx context.Context, path string) (*model.VaultStatus, error) {
	st, err := v.mgr.Open(ctx, v.g, path)
	if err != nil {
		return nil, err
	}
	if err := v.sy.Start(st.Path, SyncEmitter); err != nil {
		return nil, err
	}
	return st, nil
}

// Reopen restores the last configured vault (app startup).
func (v *VaultService) Reopen(ctx context.Context) (*model.VaultStatus, error) {
	path := v.mgr.ConfiguredPath()
	if path == "" {
		return nil, nil // fresh install: wizard
	}
	st, err := v.mgr.Open(ctx, v.g, path)
	if err != nil {
		return nil, err
	}
	if err := v.sy.Start(st.Path, SyncEmitter); err != nil {
		return nil, err
	}
	return st, nil
}

// Status summarises the configured vault.
func (v *VaultService) Status(ctx context.Context) *model.VaultStatus {
	return v.mgr.Status(ctx, v.g)
}

// AddRemote registers origin and tests connectivity (warning-only on failure).
func (v *VaultService) AddRemote(ctx context.Context, url string) error {
	if err := v.mgr.AddRemote(ctx, v.g, url); err != nil {
		return err
	}
	// Connectivity is a warning, not a failure — the remote may be offline.
	_ = git.LsRemote(ctx, v.g, v.mgr.Dir(), url)
	return nil
}

// TestRemote probes the origin remote.
func (v *VaultService) TestRemote(ctx context.Context) error {
	url := git.RemoteURL(ctx, v.g, v.mgr.Dir(), "origin")
	if url == "" {
		return errors.New("no 'origin' remote configured")
	}
	return git.LsRemote(ctx, v.g, v.mgr.Dir(), url)
}

// ListRemotes returns remote names.
func (v *VaultService) ListRemotes(ctx context.Context) []string {
	return git.Remotes(ctx, v.g, v.mgr.Dir())
}

// RemoveRemote removes a remote by name.
func (v *VaultService) RemoveRemote(ctx context.Context, name string) error {
	return v.mgr.RemoveRemote(ctx, v.g, name)
}

// ---------- SyncService ----------

// SyncService exposes sync status + manual operations.
type SyncService struct {
	mgr *vault.Manager
	sy  *sync.Syncer
}

func NewSyncService(mgr *vault.Manager, sy *sync.Syncer) *SyncService {
	return &SyncService{mgr: mgr, sy: sy}
}

// Status returns the current sync state.
func (s *SyncService) Status() model.SyncStatus {
	if s.mgr.Dir() == "" {
		return model.SyncStatus{State: model.SyncIdle}
	}
	return s.sy.Status()
}

// CommitNow commits immediately.
func (s *SyncService) CommitNow(msg string) error { return s.sy.CommitNow(msg) }

// PushNow pushes immediately.
func (s *SyncService) PushNow() error { return s.sy.PushNow() }

// PullNow rebases onto the remote.
func (s *SyncService) PullNow() error { return s.sy.PullNow() }

// History returns the git operation ring buffer (sync-error drawer).
func (s *SyncService) History() []model.GitLogEntry { return s.sy.History() }

// ---------- TreeService ----------

// TreeService exposes the collection tree (read-only in phase 1).
type TreeService struct {
	mgr *vault.Manager
}

func NewTreeService(mgr *vault.Manager) *TreeService { return &TreeService{mgr: mgr} }

// ListCollections returns all collections with their environments.
func (t *TreeService) ListCollections() ([]*model.TreeNode, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	return store.LoadCollections(dir)
}

// GetTree returns one collection tree by slug.
func (t *TreeService) GetTree(collectionID string) (*model.TreeNode, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	return store.GetTree(dir, collectionID)
}
