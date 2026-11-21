package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/store"
)

// TestImportRealExports verifies the converters against real-world exports:
// a Postman collection from the official newman repo, an Insomnia v4 export
// from the Kong insomnia repo, and a subset of the official Bruno test
// collection. These complement the hand-written fixtures above.

func TestImportRealPostmanExport(t *testing.T) {
	v := vaultFor(t)
	res, err := ImportFromPath(v, filepath.Join("testdata", "newman-sample-collection.json"), "postman", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Requests != 3 {
		t.Errorf("expected 3 requests, got %d", res.Requests)
	}
	if res.Folders != 0 || res.Environments != 0 {
		t.Errorf("unexpected folders=%d envs=%d", res.Folders, res.Environments)
	}
	if res.Warnings != nil {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
}

func TestImportRealInsomniaExport(t *testing.T) {
	v := vaultFor(t)
	res, err := ImportFromPath(v, filepath.Join("testdata", "insomnia-v4-real-export.json"), "insomnia", "")
	if err != nil {
		t.Fatal(err)
	}
	// 13 requests, one nested in the "Example Folder" request_group
	if res.Requests != 13 {
		t.Errorf("expected 13 requests, got %d", res.Requests)
	}
	if res.Folders != 1 {
		t.Errorf("expected 1 folder (request_group), got %d", res.Folders)
	}
	if res.Environments != 1 {
		t.Errorf("expected 1 environment, got %d", res.Environments)
	}
}

func TestImportRealBrunoCollection(t *testing.T) {
	v := vaultFor(t)
	res, err := ImportFromPath(v, filepath.Join("testdata", "bruno-real"), "bruno", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Requests == 0 {
		t.Fatal("expected requests from the Bruno test collection")
	}
	// every imported request file must round-trip: loadable via the store
	count := 0
	root := filepath.Join(v, res.CollectionID)
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".yml") && !strings.Contains(path, ".rapi") {
			colID, rel, splitErr := splitRelForTest(res.CollectionID, path, v)
			if splitErr != nil {
				return nil
			}
			if _, loadErr := store.LoadRequestDoc(v, colID, rel); loadErr == nil {
				count++
			}
		}
		return nil
	})
	if count < res.Requests {
		t.Errorf("loaded %d request docs, import reported %d", count, res.Requests)
	}
}

// splitRelForTest maps an absolute yaml path back to (collectionID, rel).
func splitRelForTest(colID, absPath, vault string) (string, string, error) {
	relToCol, err := filepath.Rel(filepath.Join(vault, colID), absPath)
	if err != nil {
		return "", "", err
	}
	rel := strings.TrimSuffix(relToCol, ".yml")
	rel = strings.TrimSuffix(rel, "index")
	return colID, strings.Trim(rel, "/"), nil
}
