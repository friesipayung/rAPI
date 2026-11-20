package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

// crudFixture creates an empty vault-shaped dir.
func crudFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rapi"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCollectionFolderRequestLifecycle(t *testing.T) {
	v := crudFixture(t)

	slug, err := CreateCollection(v, "Sample API")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "sample-api" {
		t.Fatalf("slug = %q", slug)
	}
	// duplicate name gets a unique slug
	slug2, _ := CreateCollection(v, "Sample API")
	if slug2 != "sample-api-2" {
		t.Fatalf("second slug = %q", slug2)
	}

	// env registered
	envs, err := ListEnvironments(v, slug)
	if err != nil || len(envs) != 1 || envs[0].Name != "local" {
		t.Fatalf("envs = %+v err=%v", envs, err)
	}

	rel, err := CreateRequest(v, slug, "", "Get Anything")
	if err != nil || rel != "get-anything" {
		t.Fatalf("rel = %q err = %v", rel, err)
	}
	frel, err := CreateFolder(v, slug, "", "Users")
	if err != nil || frel != "users" {
		t.Fatalf("folder rel = %q err = %v", frel, err)
	}
	rel2, err := CreateRequest(v, slug, frel, "Get User")
	if err != nil || rel2 != "users/get-user" {
		t.Fatalf("nested rel = %q err = %v", rel2, err)
	}

	// load + save roundtrip, examples preserved
	doc, err := LoadRequestDoc(v, slug, rel)
	if err != nil {
		t.Fatal(err)
	}
	doc.Http.Method = "POST"
	doc.Http.URL = "{{baseUrl}}/anything"
	doc.Docs = "# Docs"
	if err := SaveRequestDoc(v, slug, rel, doc); err != nil {
		t.Fatal(err)
	}
	doc2, _ := LoadRequestDoc(v, slug, rel)
	if doc2.Http.Method != "POST" || doc2.Docs != "# Docs" {
		t.Fatalf("roundtrip: %+v", doc2)
	}

	// rename keeps file
	newRel, err := Rename(v, slug, rel, "Post Anything")
	if err != nil || newRel != "post-anything" {
		t.Fatalf("rename → %q err = %v", newRel, err)
	}
	if _, err := os.Stat(filepath.Join(v, slug, "post-anything.yml")); err != nil {
		t.Fatal("renamed file missing")
	}

	// delete
	if err := DeleteTree(v, slug, frel); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v, slug, "users")); !os.IsNotExist(err) {
		t.Fatal("folder still exists")
	}
	if err := DeleteCollection(v, slug2); err != nil {
		t.Fatal(err)
	}
}

func TestEnvCRUD(t *testing.T) {
	v := crudFixture(t)
	slug, _ := CreateCollection(v, "Env Test")

	if err := SaveEnvironment(v, slug, &model.EnvironmentDoc{Name: "staging", Variables: []model.Variable{{Name: "baseUrl", Value: "https://staging.example"}}}); err != nil {
		t.Fatal(err)
	}
	envs, _ := ListEnvironments(v, slug)
	if len(envs) != 2 { // local + staging
		t.Fatalf("envs = %d", len(envs))
	}
	if err := DeleteEnvironment(v, slug, "staging"); err != nil {
		t.Fatal(err)
	}
	envs, _ = ListEnvironments(v, slug)
	if len(envs) != 1 {
		t.Fatalf("after delete envs = %d", len(envs))
	}
}

func TestUIState(t *testing.T) {
	v := crudFixture(t)
	st, _ := LoadUIState(v)
	st.ActiveEnvs["col-a"] = "staging"
	if err := SaveUIState(v, st); err != nil {
		t.Fatal(err)
	}
	st2, _ := LoadUIState(v)
	if st2.ActiveEnvs["col-a"] != "staging" {
		t.Fatalf("ui-state roundtrip: %+v", st2)
	}
}

func TestSafeRelRejectsEscape(t *testing.T) {
	v := crudFixture(t)
	if _, err := LoadRequestDoc(v, "col", "../../etc/passwd"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := DeleteCollection(v, "../escape"); err == nil {
		t.Fatal("collection traversal accepted")
	}
}
