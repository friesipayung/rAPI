package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// isolateGitConfig disables the host's global git config (e.g. GPG signing)
// so tests are fast and hermetic.
func isolateGitConfig(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, []byte("[commit]\n	gpgsign = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
}

func TestInitCommitPushPull(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	g := Cli{}
	dir := t.TempDir()

	if err := Init(ctx, g, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !IsRepo(ctx, g, dir) {
		t.Fatal("expected repo after init")
	}
	if err := EnsureIdentity(ctx, g, dir); err != nil {
		t.Fatalf("identity: %v", err)
	}

	// No commits yet.
	if HasCommits(ctx, g, dir) {
		t.Fatal("fresh repo must not have commits")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.yml"), []byte("x: 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	committed, err := Commit(ctx, g, dir, "first")
	if err != nil || !committed {
		t.Fatalf("commit: committed=%v err=%v", committed, err)
	}
	// Second commit with no changes must be a no-op.
	committed, err = Commit(ctx, g, dir, "second")
	if err != nil || committed {
		t.Fatalf("clean commit should be no-op: committed=%v err=%v", committed, err)
	}
	if !HasCommits(ctx, g, dir) {
		t.Fatal("expected commits after first commit")
	}

	// Push to a local bare remote, then pull --rebase.
	remoteDir := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", remoteDir).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	if err := RemoteAdd(ctx, g, dir, "origin", remoteDir); err != nil {
		t.Fatalf("remote add: %v", err)
	}
	if url := RemoteURL(ctx, g, dir, "origin"); url != remoteDir {
		t.Fatalf("remote url = %q", url)
	}
	if err := Push(ctx, g, dir); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := PullRebase(ctx, g, dir); err != nil {
		t.Fatalf("pull --rebase: %v", err)
	}
	if err := LsRemote(ctx, g, dir, remoteDir); err != nil {
		t.Fatalf("ls-remote: %v", err)
	}

	// Clean status.
	lines, err := StatusPorcelain(ctx, g, dir)
	if err != nil || len(lines) != 0 {
		t.Fatalf("expected clean status, got %v err=%v", lines, err)
	}
	if err := RemoteRemove(ctx, g, dir, "origin"); err != nil {
		t.Fatalf("remote remove: %v", err)
	}
}

func TestIsConflict(t *testing.T) {
	if !IsConflict("CONFLICT (content): Merge conflict in a.yml") {
		t.Fatal("expected conflict detection")
	}
	if IsConflict("everything up to date") {
		t.Fatal("false positive")
	}
}
