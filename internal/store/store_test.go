package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func fixtureVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"col-a/opencollection.yml":     "opencollection: \"1.0.0\"\ninfo:\n  name: Collection A\nconfig:\n  environments:\n    - local\ndocs: |\n  hello docs\n",
		"col-a/environments/local.yml": "name: local\n",
		"col-a/users/folder.yml":       "info:\n  name: Users\n  type: folder\n  seq: 1\n",
		"col-a/users/get-user.yml":     "info:\n  name: Get User\n  type: http\n  seq: 2\nhttp:\n  method: GET\n  url: \"{{baseUrl}}/users/1\"\nexamples:\n  - name: 200 OK\n",
		"col-a/create-user.yml":        "info:\n  name: Create User\n  type: http\n  seq: 1\nhttp:\n  method: POST\n  url: \"{{baseUrl}}/users\"\n",
		"col-b/opencollection.yml":     "opencollection: \"1.0.0\"\ninfo:\n  name: Collection B\n",
		".rapi/config.yml":             "formatVersion: \"1\"\nname: Test Vault\n",
		"not-a-collection/README.md":   "ignore me\n",
	}
	for rel, content := range files {
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadVaultConfig(t *testing.T) {
	root := fixtureVault(t)
	cfg, err := LoadVaultConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "Test Vault" || cfg.FormatVersion != "1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadCollections(t *testing.T) {
	root := fixtureVault(t)
	cols, err := LoadCollections(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 collections, got %d: %+v", len(cols), cols)
	}
	a := cols[0]
	if a.Name != "Collection A" || a.ID != "col-a" {
		t.Fatalf("bad collection node: %+v", a)
	}
	if !a.Docs {
		t.Fatal("expected docs flag on collection A")
	}
	if len(a.Envs) != 1 || a.Envs[0] != "local" {
		t.Fatalf("expected envs [local], got %v", a.Envs)
	}
	// Seq order: create-user (seq 1) before users folder (seq 1, name sort) / get-user.
	if len(a.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(a.Children))
	}
	req := a.Children[0]
	if req.Name != "Create User" || req.Method != "POST" {
		t.Fatalf("bad request node: %+v", req)
	}
	users := a.Children[1]
	if users.Type != model.NodeFolder || len(users.Children) != 1 {
		t.Fatalf("bad folder node: %+v", users)
	}
	getUser := users.Children[0]
	if getUser.Examples != 1 || getUser.Method != "GET" {
		t.Fatalf("bad nested request: %+v", getUser)
	}
}

func TestGetTree(t *testing.T) {
	root := fixtureVault(t)
	node, err := GetTree(root, "col-b")
	if err != nil {
		t.Fatal(err)
	}
	if node.Name != "Collection B" {
		t.Fatalf("bad node: %+v", node)
	}
	if _, err := GetTree(root, "missing"); err == nil {
		t.Fatal("expected error for missing collection")
	}
}
