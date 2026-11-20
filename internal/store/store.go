// Package store loads and validates vault files (OpenCollection filesystem
// layout) and builds the collection tree.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/friesipayung/rapi/internal/model"
)

// LoadVaultConfig reads <vault>/.rapi/config.yml.
func LoadVaultConfig(dir string) (*model.VaultConfig, error) {
	b, err := os.ReadFile(filepath.Join(dir, ".rapi", "config.yml"))
	if err != nil {
		return nil, err
	}
	var cfg model.VaultConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("invalid .rapi/config.yml: %w", err)
	}
	return &cfg, nil
}

// SaveVaultConfig writes <vault>/.rapi/config.yml (0600 dir, 0644 file).
func SaveVaultConfig(dir string, cfg *model.VaultConfig) error {
	if err := os.MkdirAll(filepath.Join(dir, ".rapi"), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".rapi", "config.yml"), b, 0o644)
}

// LoadCollections scans the vault and returns one node per collection, with
// folders/requests nested. Read-only and tolerant: unreadable files are
// skipped, never crash the tree.
func LoadCollections(vaultPath string) ([]*model.TreeNode, error) {
	entries, err := os.ReadDir(vaultPath)
	if err != nil {
		return nil, err
	}
	var cols []*model.TreeNode
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(vaultPath, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "opencollection.yml")); err != nil {
			continue // not a collection
		}
		col, err := loadCollection(dir, e.Name())
		if err != nil {
			continue // skip broken collections
		}
		cols = append(cols, col)
	}
	sortNodes(cols)
	return cols, nil
}

// GetTree returns a single collection tree by slug.
func GetTree(vaultPath, collectionID string) (*model.TreeNode, error) {
	dir := filepath.Join(vaultPath, collectionID)
	if _, err := os.Stat(filepath.Join(dir, "opencollection.yml")); err != nil {
		return nil, fmt.Errorf("collection %q not found", collectionID)
	}
	return loadCollection(dir, collectionID)
}

func loadCollection(dir, slug string) (*model.TreeNode, error) {
	node := &model.TreeNode{ID: slug, Type: model.NodeCollection}
	b, err := os.ReadFile(filepath.Join(dir, "opencollection.yml"))
	if err == nil {
		var cf model.CollectionFile
		if yaml.Unmarshal(b, &cf) == nil {
			node.Name = firstNonEmpty(cf.Info.Name, slug)
			node.Envs = cf.Config.Environments
			node.Docs = strings.TrimSpace(cf.Docs) != ""
		}
	}
	if node.Name == "" {
		node.Name = slug
	}
	node.Children = loadChildren(dir, slug)
	return node, nil
}

// loadChildren walks a collection/folder directory.
func loadChildren(dir, parentID string) []*model.TreeNode {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var nodes []*model.TreeNode
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "environments" {
			continue
		}
		if e.IsDir() {
			sub := filepath.Join(dir, name)
			fb, err := os.ReadFile(filepath.Join(sub, "folder.yml"))
			if err != nil {
				continue // not a folder item
			}
			var ff model.FolderFile
			if yaml.Unmarshal(fb, &ff) != nil {
				continue
			}
			id := parentID + "/" + name
			node := &model.TreeNode{
				ID:       id,
				Type:     model.NodeFolder,
				Name:     firstNonEmpty(ff.Info.Name, name),
				Seq:      ff.Info.Seq,
				Docs:     strings.TrimSpace(ff.Docs) != "",
				Children: loadChildren(sub, id),
			}
			nodes = append(nodes, node)
			continue
		}
		if !strings.HasSuffix(name, ".yml") || name == "opencollection.yml" || name == "folder.yml" {
			continue
		}
		rb, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var rf model.RequestFile
		if yaml.Unmarshal(rb, &rf) != nil {
			continue
		}
		id := parentID + "/" + strings.TrimSuffix(name, ".yml")
		node := &model.TreeNode{
			ID:       id,
			Type:     model.NodeHTTP,
			Name:     firstNonEmpty(rf.Info.Name, strings.TrimSuffix(name, ".yml")),
			Method:   rf.Http.Method,
			URL:      rf.Http.URL,
			Seq:      rf.Info.Seq,
			Docs:     strings.TrimSpace(rf.Docs) != "",
			Examples: len(rf.Examples),
		}
		nodes = append(nodes, node)
	}
	sortNodes(nodes)
	return nodes
}

func sortNodes(nodes []*model.TreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Seq != nodes[j].Seq {
			return nodes[i].Seq < nodes[j].Seq
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, n := range nodes {
		sortNodes(n.Children)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
