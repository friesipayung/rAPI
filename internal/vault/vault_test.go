package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/store"
)

func isolateGitConfig(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, []byte("[commit]\n	gpgsign = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := New()
	m.cfgFile = filepath.Join(t.TempDir(), "app.json") // never touch the real app config
	return m
}

func TestInitScaffoldsVault(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	g := git.Cli{}
	m := newTestManager(t)
	dir := filepath.Join(t.TempDir(), "vault")

	st, err := m.Init(ctx, g, dir, "My Vault", "")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Valid || st.Name != "My Vault" {
		t.Fatalf("bad status: %+v", st)
	}
	// Config + sample collection exist.
	cfg, err := store.LoadVaultConfig(dir)
	if err != nil || cfg.Name != "My Vault" {
		t.Fatalf("config: %+v err=%v", cfg, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample-api", "opencollection.yml")); err != nil {
		t.Fatalf("sample collection missing: %v", err)
	}
	// Initial commit exists.
	if !git.HasCommits(ctx, g, dir) {
		t.Fatal("expected initial commit")
	}
	// Reopen returns the same vault (idempotent init).
	st2, err := m.Init(ctx, g, dir, "Other Name", "")
	if err != nil {
		t.Fatal(err)
	}
	if st2.Name != "My Vault" {
		t.Fatalf("re-init must not overwrite config, got %q", st2.Name)
	}
	// Collections are loadable from the scaffold.
	cols, err := store.LoadCollections(dir)
	if err != nil || len(cols) != 1 {
		t.Fatalf("collections: %d err=%v", len(cols), err)
	}
}

func TestInitAdoptsExistingRepo(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	g := git.Cli{}
	m := newTestManager(t)
	dir := t.TempDir()

	// Pre-existing repo with a commit.
	if err := git.Init(ctx, g, dir); err != nil {
		t.Fatal(err)
	}
	if err := git.EnsureIdentity(ctx, g, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Commit(ctx, g, dir, "existing"); err != nil {
		t.Fatal(err)
	}

	st, err := m.Init(ctx, g, dir, "Adopted", "")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Valid {
		t.Fatalf("bad status: %+v", st)
	}
	// Existing commits preserved + scaffold committed on top.
	if _, err := git.Commit(ctx, g, dir, "probe: should be no-op"); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadVaultConfig(dir)
	if err != nil || cfg.Name != "Adopted" {
		t.Fatalf("config: %+v err=%v", cfg, err)
	}
}

func TestOpenRejectsNonVault(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	g := git.Cli{}
	m := newTestManager(t)

	// Plain repo without .rapi/config.yml.
	dir := t.TempDir()
	_ = git.Init(ctx, g, dir)
	if _, err := m.Open(ctx, g, dir); err == nil {
		t.Fatal("expected error opening a repo without vault config")
	}

	// Not a repo at all.
	if _, err := m.Open(ctx, g, t.TempDir()); err == nil {
		t.Fatal("expected error opening a non-repo")
	}
}

func TestAddRemote(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	g := git.Cli{}
	m := newTestManager(t)
	dir := filepath.Join(t.TempDir(), "vault")
	if _, err := m.Init(ctx, g, dir, "V", ""); err != nil {
		t.Fatal(err)
	}
	remote := t.TempDir() // local path acts as a remote URL for git
	if err := m.AddRemote(ctx, g, remote); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(ctx, g); !st.HasRemote || st.RemoteURL != remote {
		t.Fatalf("status: %+v", st)
	}
	if err := m.AddRemote(ctx, g, remote); err == nil {
		t.Fatal("duplicate remote must fail")
	}
	if err := m.RemoveRemote(ctx, g, "origin"); err != nil {
		t.Fatal(err)
	}
}
