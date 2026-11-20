package store

// Phase 2 CRUD: request/folder/collection/environment documents and
// per-vault UI state. All paths are vault-relative; no path escaping the
// vault is allowed (validated here once, used everywhere).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/friesipayung/rapi/internal/model"
)

// safeRel rejects paths that try to escape the vault (.., absolute, empty).
func safeRel(rel string) (string, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	clean := filepath.Clean(rel)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	return clean, nil
}

func readYAML(path string, out interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}

func writeYAML(path string, in interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// Slugify turns a display name into a filesystem-safe slug.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	return s
}

// uniqueSlug returns "base", "base-2", "base-3"... not colliding with an
// existing entry in dir.
func uniqueSlug(dir, base string) string {
	if base == "" {
		base = "untitled"
	}
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
}

// ---------- requests ----------

// LoadRequestDoc loads <collection>/<rel>.yml as a full request document.
func LoadRequestDoc(vaultPath, collectionID, rel string) (*model.RequestDoc, error) {
	rel, err := safeRel(rel)
	if err != nil {
		return nil, err
	}
	var doc model.RequestDoc
	if err := readYAML(filepath.Join(vaultPath, collectionID, rel+".yml"), &doc); err != nil {
		return nil, err
	}
	if doc.Info.Name == "" {
		doc.Info.Name = strings.TrimSuffix(filepath.Base(rel), ".yml")
	}
	return &doc, nil
}

// SaveRequestDoc writes the request back to <collection>/<rel>.yml,
// preserving examples/runtime (loaded fresh and merged).
func SaveRequestDoc(vaultPath, collectionID, rel string, doc *model.RequestDoc) error {
	rel, err := safeRel(rel)
	if err != nil {
		return err
	}
	if doc.Info.Type == "" {
		doc.Info.Type = "http"
	}
	// preserve raw sections the editor does not touch
	if prev, err := LoadRequestDoc(vaultPath, collectionID, rel); err == nil {
		if doc.Examples.Node.Kind == 0 {
			doc.Examples = prev.Examples
		}
		if doc.Runtime.Node.Kind == 0 {
			doc.Runtime = prev.Runtime
		}
	}
	return writeYAML(filepath.Join(vaultPath, collectionID, rel+".yml"), doc)
}

// ---------- collection / folder docs ----------

// LoadCollectionDoc loads opencollection.yml of a collection.
func LoadCollectionDoc(vaultPath, collectionID string) (*model.CollectionDoc, error) {
	var doc model.CollectionDoc
	if err := readYAML(filepath.Join(vaultPath, collectionID, "opencollection.yml"), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// SaveCollectionDoc updates collection-level fields (name/summary/docs).
func SaveCollectionDoc(vaultPath, collectionID string, doc *model.CollectionDoc) error {
	if doc.Opencollection == "" {
		doc.Opencollection = model.OpenCollectionVersion
	}
	return writeYAML(filepath.Join(vaultPath, collectionID, "opencollection.yml"), doc)
}

// LoadFolderDoc loads folder.yml of a folder.
func LoadFolderDoc(vaultPath, collectionID, rel string) (*model.FolderDoc, error) {
	rel, err := safeRel(rel)
	if err != nil {
		return nil, err
	}
	var doc model.FolderDoc
	if err := readYAML(filepath.Join(vaultPath, collectionID, rel, "folder.yml"), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// SaveFolderDoc writes folder.yml.
func SaveFolderDoc(vaultPath, collectionID, rel string, doc *model.FolderDoc) error {
	rel, err := safeRel(rel)
	if err != nil {
		return err
	}
	if doc.Info.Type == "" {
		doc.Info.Type = "folder"
	}
	return writeYAML(filepath.Join(vaultPath, collectionID, rel, "folder.yml"), doc)
}

// ---------- creation ----------

// CreateCollection creates <vault>/<slug>/ with opencollection.yml and a
// default `local` environment. Returns the collection slug (ID).
func CreateCollection(vaultPath, name string) (string, error) {
	slug := Slugify(name)
	if slug == "" {
		return "", fmt.Errorf("invalid collection name %q", name)
	}
	slug = uniqueSlug(vaultPath, slug)
	dir := filepath.Join(vaultPath, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	doc := &model.CollectionDoc{Opencollection: model.OpenCollectionVersion}
	doc.Info.Name = name
	doc.Info.Version = "1.0.0"
	if err := writeYAML(filepath.Join(dir, "opencollection.yml"), doc); err != nil {
		return "", err
	}
	env := &model.EnvironmentDoc{Name: "local"}
	if err := writeYAML(filepath.Join(dir, "environments", "local.yml"), env); err != nil {
		return "", err
	}
	return slug, nil
}

// CreateFolder creates <collection>/<parent>/<slug>/folder.yml.
// parentRel "" = collection root. Returns the new folder rel path.
func CreateFolder(vaultPath, collectionID, parentRel, name string) (string, error) {
	if parentRel != "" {
		var err error
		if parentRel, err = safeRel(parentRel); err != nil {
			return "", err
		}
	}
	base := filepath.Join(vaultPath, collectionID, parentRel)
	if _, err := os.Stat(filepath.Join(base, "folder.yml")); parentRel != "" && err != nil {
		return "", fmt.Errorf("parent folder not found")
	}
	slug := uniqueSlug(base, Slugify(name))
	rel := slug
	if parentRel != "" {
		rel = parentRel + "/" + slug
	}
	doc := &model.FolderDoc{}
	doc.Info.Name = name
	doc.Info.Type = "folder"
	if err := SaveFolderDoc(vaultPath, collectionID, rel, doc); err != nil {
		return "", err
	}
	return rel, nil
}

// CreateRequest creates <collection>/<parent>/<slug>.yml with a default
// GET request. Returns the request rel path (without .yml).
func CreateRequest(vaultPath, collectionID, parentRel, name string) (string, error) {
	if parentRel != "" {
		var err error
		if parentRel, err = safeRel(parentRel); err != nil {
			return "", err
		}
	}
	base := filepath.Join(vaultPath, collectionID, parentRel)
	if _, err := os.Stat(filepath.Join(base, "folder.yml")); parentRel != "" && err != nil {
		return "", fmt.Errorf("parent folder not found")
	}
	slug := uniqueSlug(base, Slugify(name))
	rel := slug
	if parentRel != "" {
		rel = parentRel + "/" + slug
	}
	doc := &model.RequestDoc{}
	doc.Info.Name = name
	doc.Info.Type = "http"
	doc.Http.Method = "GET"
	doc.Http.URL = ""
	if err := SaveRequestDoc(vaultPath, collectionID, rel, doc); err != nil {
		return "", err
	}
	return rel, nil
}

// Delete removes a request file, a folder subtree, or a whole collection.
func Delete(vaultPath, collectionID, rel string) error {
	if rel == "" {
		return fmt.Errorf("refusing to delete collection root via rel; use DeleteCollection")
	}
	rel, err := safeRel(rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(vaultPath, collectionID, rel+".yml"))
}

// DeleteTree removes a folder subtree or a request file whether or not it
// has an extension (folder = directory).
func DeleteTree(vaultPath, collectionID, rel string) error {
	rel, err := safeRel(rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(vaultPath, collectionID, rel))
}

// DeleteCollection removes the whole collection directory.
func DeleteCollection(vaultPath, collectionID string) error {
	if collectionID == "" || collectionID == "." || strings.Contains(collectionID, "/") {
		return fmt.Errorf("invalid collection id %q", collectionID)
	}
	return os.RemoveAll(filepath.Join(vaultPath, collectionID))
}

// Rename renames a request file (rel may end in .yml) or a folder dir.
// Returns the new rel path.
func Rename(vaultPath, collectionID, rel, newName string) (string, error) {
	rel, err := safeRel(rel)
	if err != nil {
		return "", err
	}
	slug := Slugify(newName)
	if slug == "" {
		return "", fmt.Errorf("invalid name %q", newName)
	}
	old := filepath.Join(vaultPath, collectionID, rel)
	isFile := false
	if _, err := os.Stat(old + ".yml"); err == nil {
		isFile = true
		old += ".yml"
	}
	dir, _ := filepath.Split(rel)
	newPath := dir + slug
	if isFile {
		newPath += ".yml"
	}
	dst := filepath.Join(vaultPath, collectionID, newPath)
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("%q already exists", slug)
	}
	if err := os.Rename(old, dst); err != nil {
		return "", err
	}
	if isFile {
		newPath = strings.TrimSuffix(newPath, ".yml")
	}
	return filepath.ToSlash(newPath), nil
}

// ---------- environments ----------

// ListEnvironments returns all environment docs of a collection, ordered by
// config.environments, then anything on disk not yet registered.
func ListEnvironments(vaultPath, collectionID string) ([]*model.EnvironmentDoc, error) {
	col, err := LoadCollectionDoc(vaultPath, collectionID)
	if err != nil {
		return nil, err
	}
	envDir := filepath.Join(vaultPath, collectionID, "environments")
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return []*model.EnvironmentDoc{}, nil
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		onDisk[strings.TrimSuffix(e.Name(), ".yml")] = true
	}
	var docs []*model.EnvironmentDoc
	seen := map[string]bool{}
	add := func(slug string) {
		if seen[slug] || !onDisk[slug] {
			return
		}
		var doc model.EnvironmentDoc
		if readYAML(filepath.Join(envDir, slug+".yml"), &doc) != nil {
			return
		}
		seen[slug] = true
		docs = append(docs, &doc)
	}
	for _, name := range col.Config.Environments {
		add(Slugify(name))
	}
	for slug := range onDisk {
		add(slug)
	}
	return docs, nil
}

// SaveEnvironment writes environments/<slug>.yml and registers it in
// config.environments.
func SaveEnvironment(vaultPath, collectionID string, env *model.EnvironmentDoc) error {
	if env.Name == "" {
		return fmt.Errorf("environment name required")
	}
	slug := Slugify(env.Name)
	if err := writeYAML(filepath.Join(vaultPath, collectionID, "environments", slug+".yml"), env); err != nil {
		return err
	}
	col, err := LoadCollectionDoc(vaultPath, collectionID)
	if err != nil {
		return err
	}
	registered := false
	for _, n := range col.Config.Environments {
		if Slugify(n) == slug {
			registered = true
			break
		}
	}
	if !registered {
		col.Config.Environments = append(col.Config.Environments, env.Name)
		if err := SaveCollectionDoc(vaultPath, collectionID, col); err != nil {
			return err
		}
	}
	return nil
}

// DeleteEnvironment removes the env file and unregisters it.
func DeleteEnvironment(vaultPath, collectionID, name string) error {
	slug := Slugify(name)
	if err := os.Remove(filepath.Join(vaultPath, collectionID, "environments", slug+".yml")); err != nil && !os.IsNotExist(err) {
		return err
	}
	col, err := LoadCollectionDoc(vaultPath, collectionID)
	if err != nil {
		return err
	}
	kept := col.Config.Environments[:0]
	for _, n := range col.Config.Environments {
		if Slugify(n) != slug {
			kept = append(kept, n)
		}
	}
	col.Config.Environments = kept
	return SaveCollectionDoc(vaultPath, collectionID, col)
}

// ---------- ui state ----------

// LoadUIState reads .rapi/ui-state.yml (zero value when absent).
func LoadUIState(vaultPath string) (*model.UIState, error) {
	st := &model.UIState{ActiveEnvs: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(vaultPath, ".rapi", "ui-state.yml"))
	if err != nil {
		return st, nil // absent is fine
	}
	if err := yaml.Unmarshal(b, st); err != nil {
		return st, nil
	}
	if st.ActiveEnvs == nil {
		st.ActiveEnvs = map[string]string{}
	}
	return st, nil
}

// SaveUIState writes .rapi/ui-state.yml.
func SaveUIState(vaultPath string, st *model.UIState) error {
	if st.ActiveEnvs == nil {
		st.ActiveEnvs = map[string]string{}
	}
	return writeYAML(filepath.Join(vaultPath, ".rapi", "ui-state.yml"), st)
}

// CountExamples returns how many examples a raw YAML node holds (0 otherwise).
func CountExamples(n model.YamlRaw) int {
	if n.Node.Kind == yaml.SequenceNode {
		return len(n.Node.Content)
	}
	return 0
}

// SortStrings is a tiny helper for deterministic env listings.
func SortStrings(s []string) { sort.Strings(s) }
