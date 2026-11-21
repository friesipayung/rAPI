package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/importer"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
)

func expVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rapi"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExportPostmanFromRealFixture(t *testing.T) {
	v := expVault(t)
	fixture, err := filepath.Abs("../importer/testdata/newman-sample-collection.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := importer.ImportFromPath(v, fixture, "postman", "")
	if err != nil {
		t.Fatal(err)
	}

	data, _, err := Collect(v, res.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"`) {
		t.Fatal("missing v2.1 schema")
	}
	if !strings.Contains(string(data), `"request"`) {
		t.Fatal("no requests in export")
	}

	// round-trip: the export must re-import as Postman with the same counts
	rtPath := filepath.Join(t.TempDir(), "roundtrip.json")
	if err := os.WriteFile(rtPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := importer.Detect(rtPath); got != "postman" {
		t.Fatalf("re-import detect = %q", got)
	}
	res2, err := importer.ImportFromPath(v, rtPath, "postman", "")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Requests != res.Requests || res2.Folders != res.Folders {
		t.Fatalf("round-trip mismatch: %d req/%d folders vs %d req/%d folders",
			res2.Requests, res2.Folders, res.Requests, res.Folders)
	}

	// tree of the re-imported copy matches request names
	tree, err := store.GetTree(v, res2.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != res2.Requests+res2.Folders {
		t.Fatalf("children = %d", len(tree.Children))
	}
}

func TestExportAuthAndBody(t *testing.T) {
	v := expVault(t)
	if _, err := store.CreateCollection(v, "Col"); err != nil {
		t.Fatal(err)
	}
	rd := &model.RequestDoc{}
	rd.Info.Name = "mixed"
	rd.Info.Type = "http"
	rd.Http.Method = "POST"
	rd.Http.URL = "https://api.example.com/x"
	rd.Http.Headers = []model.KV{{Name: "Accept", Value: "application/json"}}
	rd.Http.Auth = &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{Username: "u", Password: "p"}}
	rd.Http.Body = &model.Body{Type: "json", Data: `{"a":1}`}
	rel, cerr := store.CreateRequest(v, "col", "", "mixed")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if err := store.SaveRequestDoc(v, "col", rel, rd); err != nil {
		t.Fatal(err)
	}

	data, _, err := Collect(v, "col")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"type": "basic"`, `"language": "json"`, `"method": "POST"`, `"key": "username"`, `"value": "u"`} {
		if !strings.Contains(s, want) {
			t.Errorf("export missing %s", want)
		}
	}
}
