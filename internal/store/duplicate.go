package store

// Duplicate helpers: copy a request, folder subtree or whole collection.
// Copies live next to the original with " (copy)" appended to the name.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func dupName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Copy"
	}
	return name + " (copy)"
}

// DuplicateRequest copies one request file. Returns the new rel path.
func DuplicateRequest(vaultPath, collectionID, rel string) (string, error) {
	rel, err := safeRel(rel)
	if err != nil {
		return "", err
	}
	doc, err := LoadRequestDoc(vaultPath, collectionID, rel)
	if err != nil {
		return "", fmt.Errorf("load source request: %w", err)
	}
	parent := ""
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		parent = rel[:i]
	}
	newRel, err := CreateRequestWithData(vaultPath, collectionID, parent, dupName(doc.Info.Name), &doc.Http)
	if err != nil {
		return "", err
	}
	// preserve docs, runtime (scripts/assertions), settings, examples
	saved, err := LoadRequestDoc(vaultPath, collectionID, newRel)
	if err != nil {
		return "", err
	}
	saved.Docs = doc.Docs
	saved.Runtime = doc.Runtime
	saved.Settings = doc.Settings
	saved.Examples = doc.Examples
	saved.Info.Seq = doc.Info.Seq
	if err := SaveRequestDoc(vaultPath, collectionID, newRel, saved); err != nil {
		return "", err
	}
	return newRel, nil
}

// DuplicateFolder copies a folder subtree. Returns the new rel path.
func DuplicateFolder(vaultPath, collectionID, rel string) (string, error) {
	rel, err := safeRel(rel)
	if err != nil {
		return "", err
	}
	src := filepath.Join(vaultPath, collectionID, rel)
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%q is not a folder", rel)
	}
	doc, err := LoadFolderDoc(vaultPath, collectionID, rel)
	if err != nil {
		return "", fmt.Errorf("load source folder: %w", err)
	}
	parent := ""
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		parent = rel[:i]
	}
	newName := dupName(doc.Info.Name)
	base := filepath.Join(vaultPath, collectionID, parent)
	slug := uniqueSlug(base, Slugify(newName))
	newRel := slug
	if parent != "" {
		newRel = parent + "/" + slug
	}
	dst := filepath.Join(vaultPath, collectionID, newRel)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		return "", fmt.Errorf("copy folder tree: %w", err)
	}
	// rewrite the top folder name (nested items keep their names)
	if err := rewriteFolderName(vaultPath, collectionID, newRel, newName); err != nil {
		return "", err
	}
	return newRel, nil
}

func rewriteFolderName(vaultPath, collectionID, rel, name string) error {
	doc, err := LoadFolderDoc(vaultPath, collectionID, rel)
	if err != nil {
		return err
	}
	doc.Info.Name = name
	return SaveFolderDoc(vaultPath, collectionID, rel, doc)
}

// DuplicateCollection copies a whole collection directory. Returns the new slug.
func DuplicateCollection(vaultPath, collectionID string) (string, error) {
	src := filepath.Join(vaultPath, collectionID)
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("collection %q not found", collectionID)
	}
	doc, err := LoadCollectionDoc(vaultPath, collectionID)
	if err != nil {
		return "", fmt.Errorf("load source collection: %w", err)
	}
	newName := dupName(doc.Info.Name)
	slug := uniqueSlug(vaultPath, Slugify(newName))
	if err := os.MkdirAll(filepath.Join(vaultPath, slug), 0o755); err != nil {
		return "", err
	}
	if err := os.CopyFS(filepath.Join(vaultPath, slug), os.DirFS(src)); err != nil {
		return "", fmt.Errorf("copy collection tree: %w", err)
	}
	// the copy carries the source's opencollection.yml — rewrite its name
	cd, err := LoadCollectionDoc(vaultPath, slug)
	if err != nil {
		return "", err
	}
	cd.Info.Name = newName
	if err := SaveCollectionDoc(vaultPath, slug, cd); err != nil {
		return "", err
	}
	return slug, nil
}
