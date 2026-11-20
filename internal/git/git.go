// Package git wraps the git CLI (ADR A1): hosting-agnostic auth through the
// host's own SSH keys / credential helpers; rAPI never stores tokens.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Gitter is the seam that lets go-git replace the CLI later (mobile builds).
type Gitter interface {
	// Run executes git <args...> in dir and returns stdout, stderr, err.
	Run(ctx context.Context, dir string, args ...string) (string, string, error)
}

// Cli runs the real git binary.
type Cli struct{}

// Run implements Gitter.
func (Cli) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(ctx context.Context, g Gitter, dir string) bool {
	_, _, err := g.Run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// HasCommits reports whether HEAD exists.
func HasCommits(ctx context.Context, g Gitter, dir string) bool {
	_, _, err := g.Run(ctx, dir, "rev-parse", "--verify", "HEAD")
	return err == nil
}

// Init creates a git repo in dir (no error when it already is one).
func Init(ctx context.Context, g Gitter, dir string) error {
	if IsRepo(ctx, g, dir) {
		return nil
	}
	_, stderr, err := g.Run(ctx, dir, "init")
	if err != nil {
		return wrap("init", stderr, err)
	}
	return nil
}

// EnsureIdentity sets a repo-local user identity when the host has none, so
// auto-commits never fail on a fresh machine.
func EnsureIdentity(ctx context.Context, g Gitter, dir string) error {
	_, stdout, err := g.Run(ctx, dir, "config", "user.email")
	if err == nil && strings.TrimSpace(stdout) != "" {
		return nil
	}
	if _, stderr, err := g.Run(ctx, dir, "config", "user.name", "rAPI"); err != nil {
		return wrap("config user.name", stderr, err)
	}
	if _, stderr, err := g.Run(ctx, dir, "config", "user.email", "rapi@local"); err != nil {
		return wrap("config user.email", stderr, err)
	}
	return nil
}

// StatusPorcelain returns `git status --porcelain` lines (empty = clean).
func StatusPorcelain(ctx context.Context, g Gitter, dir string) ([]string, error) {
	stdout, stderr, err := g.Run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return nil, wrap("status", stderr, err)
	}
	lines := nonEmpty(stdout)
	return lines, nil
}

// Commit stages everything and commits; returns true when a commit was made.
// If GPG signing fails (e.g. no pinentry available), it retries unsigned —
// reliability of auto-commits beats signing, and the fallback is visible in
// the sync log.
func Commit(ctx context.Context, g Gitter, dir, msg string) (bool, error) {
	changed, err := StatusPorcelain(ctx, g, dir)
	if err != nil {
		return false, err
	}
	if len(changed) == 0 {
		return false, nil
	}
	if _, stderr, err := g.Run(ctx, dir, "add", "-A"); err != nil {
		return false, wrap("add -A", stderr, err)
	}
	args := []string{"commit", "-m", msg}
	_, stderr, err := g.Run(ctx, dir, args...)
	if err != nil && strings.Contains(strings.ToLower(stderr), "gpg failed to sign") {
		args = append([]string{"-c", "commit.gpgsign=false"}, args...)
		_, stderr, err = g.Run(ctx, dir, args...)
	}
	if err != nil {
		return false, wrap("commit", stderr, err)
	}
	return true, nil
}

// RemoteURL returns the URL of remote `origin` ("" when absent).
func RemoteURL(ctx context.Context, g Gitter, dir, name string) string {
	stdout, _, err := g.Run(ctx, dir, "remote", "get-url", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stdout)
}

// Remotes lists configured remote names.
func Remotes(ctx context.Context, g Gitter, dir string) []string {
	stdout, _, err := g.Run(ctx, dir, "remote")
	if err != nil {
		return nil
	}
	return nonEmpty(stdout)
}

// RemoteAdd registers a remote.
func RemoteAdd(ctx context.Context, g Gitter, dir, name, url string) error {
	_, stderr, err := g.Run(ctx, dir, "remote", "add", name, url)
	if err != nil {
		return wrap("remote add", stderr, err)
	}
	return nil
}

// RemoteRemove removes a remote.
func RemoteRemove(ctx context.Context, g Gitter, dir, name string) error {
	_, stderr, err := g.Run(ctx, dir, "remote", "remove", name)
	if err != nil {
		return wrap("remote remove", stderr, err)
	}
	return nil
}

// RemoteHasCommits reports whether the origin remote has any refs (an empty
// freshly-created remote has none and must not be pulled from).
func RemoteHasCommits(ctx context.Context, g Gitter, dir string) bool {
	stdout, _, err := g.Run(ctx, dir, "ls-remote", "origin")
	return err == nil && strings.TrimSpace(stdout) != ""
}

// PullRebase runs `git pull --rebase --autostash` for the current branch
// (explicit origin + branch so the very first sync works before an upstream
// exists); conflict details land in stderr.
func PullRebase(ctx context.Context, g Gitter, dir string) error {
	branch, _, err := g.Run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	var stderr string
	if err == nil && branch != "" && branch != "HEAD" {
		_, stderr, err = g.Run(ctx, dir, "pull", "--rebase", "--autostash", "origin", branch)
	} else {
		_, stderr, err = g.Run(ctx, dir, "pull", "--rebase", "--autostash")
	}
	if err != nil {
		return wrap("pull --rebase", stderr, err)
	}
	return nil
}

// Push pushes the current branch to origin, setting upstream when missing
// (first push on a fresh vault; harmless afterwards).
func Push(ctx context.Context, g Gitter, dir string) error {
	_, stderr, err := g.Run(ctx, dir, "push", "-u", "origin", "HEAD")
	if err != nil {
		return wrap("push", stderr, err)
	}
	return nil
}

// LsRemote contacts a remote to test connectivity/auth.
func LsRemote(ctx context.Context, g Gitter, dir, remote string) error {
	_, stderr, err := g.Run(ctx, dir, "ls-remote", remote, "HEAD")
	if err != nil {
		return wrap("ls-remote", stderr, err)
	}
	return nil
}

// IsConflict reports whether a git failure was a rebase/merge conflict.
func IsConflict(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "conflict") ||
		strings.Contains(s, "could not apply") ||
		strings.Contains(s, "you need to resolve")
}

func nonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func wrap(op, stderr string, err error) error {
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		return fmt.Errorf("git %s: %w", op, err)
	}
	return errors.New("git " + op + ": " + detail)
}
