// Package model holds the shared data types of rAPI: vault config, loose
// parse structs for OpenCollection filesystem files, and the tree shapes
// returned to the frontend.
package model

import (
	"time"
)

const (
	// FormatVersion is the vault (.rapi/config.yml) format version.
	FormatVersion = "1"
	// OpenCollectionVersion is the OpenCollection spec version rAPI writes.
	OpenCollectionVersion = "1.0.0"
)

// ---------- vault config ----------

// VaultConfig is the content of <vault>/.rapi/config.yml.
type VaultConfig struct {
	FormatVersion string    `yaml:"formatVersion" json:"formatVersion"`
	Name          string    `yaml:"name" json:"name"`
	CreatedAt     time.Time `yaml:"createdAt" json:"createdAt"`
}

// ---------- status / sync ----------

// Sync status states.
const (
	SyncIdle     = "idle"
	SyncPending  = "pending"
	SyncSyncing  = "syncing"
	SyncSynced   = "synced"
	SyncError    = "error"
	SyncConflict = "conflict"
)

// SyncStatus is emitted to the frontend on every sync-state change.
type SyncStatus struct {
	State  string    `json:"state"`
	Detail string    `json:"detail,omitempty"`
	At     time.Time `json:"at"`
}

// GitLogEntry is one git operation kept in the ring buffer for the
// sync-error drawer (verbatim stderr, see architecture §12).
type GitLogEntry struct {
	Time   time.Time `json:"time"`
	Op     string    `json:"op"`
	Stderr string    `json:"stderr"`
	Err    string    `json:"err,omitempty"`
}

// VaultStatus summarises the vault for the wizard and the app shell.
type VaultStatus struct {
	Configured bool     `json:"configured"`
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Valid      bool     `json:"valid"`
	HasRemote  bool     `json:"hasRemote"`
	RemoteURL  string   `json:"remoteUrl,omitempty"`
	Remotes    []string `json:"remotes,omitempty"`
}

// ---------- loose parse structs ----------
// These are intentionally permissive (non-strict YAML): unknown fields are
// ignored so the app keeps working when the spec grows.

// ItemInfo is the shared `info:` block of request/folder files.
type ItemInfo struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type,omitempty"`
	Seq         int    `yaml:"seq" json:"seq"`
	Description string `yaml:"description" json:"description,omitempty"`
}

// HttpStub is the subset of `http:` the tree view needs.
type HttpStub struct {
	Method string `yaml:"method"`
	URL    string `yaml:"url"`
}

// ExampleStub is the subset of an example the tree needs (count/name only).
type ExampleStub struct {
	Name string `yaml:"name"`
}

// RequestFile is the loose shape of <request>.yml.
type RequestFile struct {
	Info     ItemInfo      `yaml:"info"`
	Http     HttpStub      `yaml:"http"`
	Docs     string        `yaml:"docs"`
	Examples []ExampleStub `yaml:"examples"`
}

// FolderFile is the loose shape of folder.yml.
type FolderFile struct {
	Info ItemInfo `yaml:"info"`
	Docs string   `yaml:"docs"`
}

// CollectionFile is the loose shape of opencollection.yml.
type CollectionFile struct {
	Opencollection string `yaml:"opencollection"`
	Info           struct {
		Name    string `yaml:"name"`
		Summary string `yaml:"summary"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Config struct {
		Environments []string `yaml:"environments"`
	} `yaml:"config"`
	Docs string `yaml:"docs"`
}

// EnvironmentFile is the loose shape of environments/<name>.yml.
type EnvironmentFile struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}

// ---------- tree ----------

// Node types in the collection tree.
const (
	NodeCollection = "collection"
	NodeFolder     = "folder"
	NodeHTTP       = "http"
)

// TreeNode is one row of the collection tree.
type TreeNode struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Name     string      `json:"name"`
	Method   string      `json:"method,omitempty"`
	URL      string      `json:"url,omitempty"`
	Seq      int         `json:"seq"`
	Docs     bool        `json:"docs,omitempty"`
	Examples int         `json:"examples,omitempty"`
	Envs     []string    `json:"envs,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}
