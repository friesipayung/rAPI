package importer

// Loose validation of OpenCollection filesystem layout: every collection has
// a valid opencollection.yml, request files parse with method+URL, folders
// have folder.yml. Reports warnings; never mutates the vault.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/friesipayung/rapi/internal/model"
)

// Report is the validation outcome for one collection.
type Report struct {
	CollectionID string   `json:"collectionId"`
	Requests     int      `json:"requests"`
	Folders      int      `json:"folders"`
	OK           bool     `json:"ok"`
	Problems     []string `json:"problems,omitempty"`
}

func (r *Report) problemf(format string, args ...any) {
	r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
	r.OK = false
}

// ValidateCollection checks the layout of one collection.
func ValidateCollection(vaultPath, collectionID string) *Report {
	rep := &Report{CollectionID: collectionID, OK: true}
	base := filepath.Join(vaultPath, collectionID)

	if _, err := os.Stat(filepath.Join(base, "opencollection.yml")); err != nil {
		rep.problemf("missing opencollection.yml")
		return rep
	}
	var col model.CollectionDoc
	b, err := os.ReadFile(filepath.Join(base, "opencollection.yml"))
	if err != nil || yaml.Unmarshal(b, &col) != nil {
		rep.problemf("opencollection.yml is not valid YAML")
		return rep
	}
	if col.Opencollection == "" {
		rep.problemf("opencollection.yml: missing `opencollection:` version field")
	}
	walkValidate(base, "", rep)
	return rep
}

func walkValidate(base, rel string, rep *Report) {
	dir := filepath.Join(base, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		rep.problemf("%s: unreadable directory", display(rel))
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		if e.IsDir() {
			if name == "environments" {
				validateEnvs(base, rep)
				continue
			}
			fb, err := os.ReadFile(filepath.Join(base, childRel, "folder.yml"))
			if err != nil {
				rep.problemf("%s: folder without folder.yml", display(childRel))
				continue
			}
			var ff model.FolderDoc
			if yaml.Unmarshal(fb, &ff) != nil {
				rep.problemf("%s/folder.yml: invalid YAML", display(childRel))
				continue
			}
			rep.Folders++
			walkValidate(base, childRel, rep)
			continue
		}
		if !strings.HasSuffix(name, ".yml") || name == "folder.yml" || name == "opencollection.yml" {
			continue
		}
		rb, err := os.ReadFile(filepath.Join(base, childRel))
		if err != nil {
			rep.problemf("%s: unreadable", display(childRel))
			continue
		}
		var rf model.RequestDoc
		if yaml.Unmarshal(rb, &rf) != nil {
			rep.problemf("%s: invalid YAML", display(childRel))
			continue
		}
		rep.Requests++
		if rf.Http.Method == "" {
			rep.problemf("%s: missing http.method", display(childRel))
		}
		if rf.Http.URL == "" {
			rep.problemf("%s: missing http.url", display(childRel))
		}
		// typed examples check (phase 3)
		for i, ex := range rf.ExamplesAsList() {
			if ex.Name == "" {
				rep.problemf("%s: example #%d has no name", display(childRel), i+1)
			}
			if ex.Response != nil && ex.Response.Status == 0 {
				rep.problemf("%s: example %q has no response status", display(childRel), ex.Name)
			}
		}
	}
}

func validateEnvs(base string, rep *Report) {
	entries, _ := os.ReadDir(filepath.Join(base, "environments"))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(base, "environments", e.Name()))
		var env model.EnvironmentDoc
		if err != nil || yaml.Unmarshal(b, &env) != nil {
			rep.problemf("environments/%s: invalid YAML", e.Name())
			continue
		}
		if env.Name == "" {
			rep.problemf("environments/%s: missing name", e.Name())
		}
	}
}

func display(rel string) string {
	if rel == "" {
		return "collection root"
	}
	return rel
}
