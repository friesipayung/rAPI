package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func dupVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rapi"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDuplicateRequest(t *testing.T) {
	v := dupVault(t)
	rel, err := CreateRequestWithData(v, "col", "", "get-user", &model.HttpRequest{
		Method: "POST", URL: "https://x.test/u",
		Headers: []model.KV{{Name: "Accept", Value: "application/json"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// enrich with docs + runtime to verify they survive the copy
	doc, _ := LoadRequestDoc(v, "col", rel)
	doc.Docs = "# docs here"
	if err := SaveRequestDoc(v, "col", rel, doc); err != nil {
		t.Fatal(err)
	}

	newRel, err := DuplicateRequest(v, "col", rel)
	if err != nil {
		t.Fatal(err)
	}
	if newRel == rel {
		t.Fatal("copy must be a new rel path")
	}
	cp, err := LoadRequestDoc(v, "col", newRel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cp.Info.Name, "(copy)") {
		t.Fatalf("name = %q", cp.Info.Name)
	}
	if cp.Http.Method != "POST" || cp.Http.URL != "https://x.test/u" {
		t.Fatalf("http not copied: %+v", cp.Http)
	}
	if cp.Docs != "# docs here" {
		t.Fatalf("docs lost: %q", cp.Docs)
	}
	if len(cp.Http.Headers) != 1 || cp.Http.Headers[0].Name != "Accept" {
		t.Fatalf("headers not copied: %+v", cp.Http.Headers)
	}
}

func TestDuplicateFolder(t *testing.T) {
	v := dupVault(t)
	if _, err := CreateCollection(v, "Col"); err != nil {
		t.Fatal(err)
	}
	frel, err := CreateFolder(v, "col", "", "Auth")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateRequest(v, "col", frel, "login"); err != nil {
		t.Fatal(err)
	}
	sub, err := CreateFolder(v, "col", frel, "nested")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateRequest(v, "col", sub, "refresh"); err != nil {
		t.Fatal(err)
	}

	newRel, err := DuplicateFolder(v, "col", frel)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := GetTree(v, "col")
	if err != nil {
		t.Fatal(err)
	}
	var found int
	var walk func(ns []*model.TreeNode)
	walk = func(ns []*model.TreeNode) {
		for _, n := range ns {
			// tree node IDs are collection-prefixed: col/auth-copy/login
			if n.ID == "col/"+newRel || strings.HasPrefix(n.ID, "col/"+newRel+"/") {
				found++
			}
			walk(n.Children)
		}
	}
	walk(tree.Children)
	if found != 4 { // folder + login + nested + refresh
		t.Fatalf("expected 4 nodes under copy, got %d", found)
	}
}

func TestDuplicateCollection(t *testing.T) {
	v := dupVault(t)
	if _, err := CreateCollection(v, "Col"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateRequest(v, "col", "", "list-pets"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEnvironment(v, "col", &model.EnvironmentDoc{Name: "prod", Variables: []model.Variable{{Name: "host", Value: "prod.example"}}}); err != nil {
		t.Fatal(err)
	}

	slug, err := DuplicateCollection(v, "col")
	if err != nil {
		t.Fatal(err)
	}
	if slug == "col" {
		t.Fatal("copy must be a new slug")
	}
	cd, err := LoadCollectionDoc(v, slug)
	if err != nil {
		t.Fatal(err)
	}
	if cd.Info.Name != "Col (copy)" {
		t.Fatalf("name = %q", cd.Info.Name)
	}
	envs, err := ListEnvironments(v, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 { // local (from CreateCollection is removed) + prod
		t.Fatalf("envs = %d (%v)", len(envs), envs)
	}
	tree, _ := GetTree(v, slug)
	if len(tree.Children) != 1 || tree.Children[0].Name != "list-pets" {
		t.Fatalf("requests not copied: %+v", tree.Children)
	}
}
