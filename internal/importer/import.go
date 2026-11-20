package importer

// Import converts third-party collections into an OpenCollection collection
// directory inside the vault. Formats: Postman v2.1, Bruno (.bru tree),
// Insomnia v4 export.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
)

// Result reports what an import created.
type Result struct {
	CollectionID string   `json:"collectionId"`
	Collections  int      `json:"collections"`
	Folders      int      `json:"folders"`
	Requests     int      `json:"requests"`
	Environments int      `json:"environments"`
	Warnings     []string `json:"warnings,omitempty"`
}

func (r *Result) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Detect guesses the source format from path/content.
func Detect(path string) string {
	// Bruno: directory containing .bru files (must be checked first —
	// ReadFile on a directory always fails)
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		if hasBruFiles(path) {
			return "bruno"
		}
		return "unknown"
	}
	if strings.HasSuffix(path, ".bru") {
		return "bruno"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "{") {
		var probe struct {
			Info struct {
				Schema string `json:"schema"`
			} `json:"info"`
			Resources []struct {
				Type string `json:"_type"`
			} `json:"resources"`
		}
		if parseJSON(b, &probe) == nil {
			if strings.Contains(probe.Info.Schema, "getpostman.com") || strings.Contains(probe.Info.Schema, "postman") {
				return "postman"
			}
			for _, r := range probe.Resources {
				// Insomnia v4 export: a resources graph, optionally wrapped in
				// an _type:"export" container
				if r.Type == "export" || r.Type == "workspace" || r.Type == "request" || r.Type == "folder" || r.Type == "environment" {
					return "insomnia"
				}
			}
		}
	}
	if strings.HasSuffix(path, ".json") {
		return "postman" // best guess
	}
	return "unknown"
}

func hasBruFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bru") {
			return true
		}
		if e.IsDir() && hasBruFiles(filepath.Join(dir, e.Name())) {
			return true
		}
	}
	return false
}

// tree is the intermediate representation every converter produces.
type tree struct {
	name     string
	children []*tree // folders
	requests []*model.RequestDoc
	vars     []model.Variable
}

// ImportFromPath converts and writes a new collection into the vault.
func ImportFromPath(vaultPath, sourcePath, format, name string) (*Result, error) {
	res := &Result{}
	var t *tree
	var err error

	switch format {
	case "", "auto":
		format = Detect(sourcePath)
	}
	switch format {
	case "postman":
		t, err = parsePostman(sourcePath)
	case "bruno":
		t, err = parseBruno(sourcePath)
	case "insomnia":
		t, err = parseInsomnia(sourcePath)
	default:
		return nil, fmt.Errorf("cannot detect format of %q (supported: Postman v2.1 JSON, Bruno .bru, Insomnia v4 JSON)", filepath.Base(sourcePath))
	}
	if err != nil {
		return nil, err
	}
	if name != "" {
		t.name = name
	}
	if t == nil || (len(t.requests) == 0 && len(t.children) == 0) {
		return nil, fmt.Errorf("no requests found in %q", filepath.Base(sourcePath))
	}

	slug, err := store.CreateCollection(vaultPath, t.name)
	if err != nil {
		return nil, err
	}
	res.CollectionID = slug
	res.Collections = 1
	if len(t.vars) > 0 {
		saveEnvFromVars(vaultPath, slug, model.EnvironmentDoc{Name: "imported", Variables: t.vars}, res)
	}
	writeNode(vaultPath, slug, "", t, res)
	return res, nil
}

// saveEnvFromVars writes an env and registers it, recording warnings softly.
func saveEnvFromVars(vaultPath, colID string, env model.EnvironmentDoc, res *Result) {
	if err := store.SaveEnvironment(vaultPath, colID, &env); err != nil {
		res.warnf("environment %q: %v", env.Name, err)
		return
	}
	res.Environments++
}

// writeNode writes one tree level (folder or collection root).
func writeNode(vaultPath, colID, parentRel string, n *tree, res *Result) {
	for _, ch := range n.children {
		frel, err := store.CreateFolder(vaultPath, colID, parentRel, ch.name)
		if err != nil {
			res.warnf("folder %q: %v", ch.name, err)
			continue
		}
		res.Folders++
		if len(ch.vars) > 0 {
			saveEnvFromVars(vaultPath, colID, model.EnvironmentDoc{Name: sanitizeEnvName(ch.name), Variables: ch.vars}, res)
		}
		writeNode(vaultPath, colID, frel, ch, res)
	}
	// deterministic order for seq
	reqs := n.requests
	sort.SliceStable(reqs, func(i, j int) bool { return reqs[i].Info.Name < reqs[j].Info.Name })
	for i, rd := range reqs {
		// CreateRequest returns the full vault-relative id (e.g. "users/get-user")
		slug, err := store.CreateRequest(vaultPath, colID, parentRel, rd.Info.Name)
		if err != nil {
			res.warnf("request %q: %v", rd.Info.Name, err)
			continue
		}
		rd.Info.Seq = i + 1
		if err := store.SaveRequestDoc(vaultPath, colID, slug, rd); err != nil {
			res.warnf("request %q: %v", rd.Info.Name, err)
			continue
		}
		res.Requests++
	}
}

func sanitizeEnvName(name string) string {
	s := store.Slugify(name)
	if s == "" {
		return "folder-env"
	}
	return s
}
