// Package vault manages the vault lifecycle: scaffold, open, status, remotes.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
)

// ErrNoVault is returned when no vault is configured/open.
var ErrNoVault = errors.New("no vault configured")

// Manager tracks the configured and active vault.
type Manager struct {
	mu      sync.Mutex
	dir     string
	cfgFile string
}

// New creates a Manager backed by ~/.config/rapi/app.json (via xdg).
func New() *Manager {
	return &Manager{cfgFile: filepath.Join(xdg.ConfigHome, "rapi", "app.json")}
}

// appConfig is the app-level config (which vault to open) — intentionally
// outside the vault: the vault belongs to git, this file just remembers the
// last opened path.
type appConfig struct {
	VaultPath string `json:"vaultPath"`
}

func (m *Manager) loadAppConfig() appConfig {
	var c appConfig
	if b, err := os.ReadFile(m.cfgFile); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func (m *Manager) saveAppConfig(c appConfig) error {
	if err := os.MkdirAll(filepath.Dir(m.cfgFile), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgFile, b, 0o600)
}

// ConfiguredPath returns the remembered vault path ("" when none).
func (m *Manager) ConfiguredPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dir != "" {
		return m.dir
	}
	return m.loadAppConfig().VaultPath
}

// Dir returns the active vault directory ("" when none open).
func (m *Manager) Dir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dir
}

// SetDir marks a vault as active and persists it.
func (m *Manager) SetDir(dir string) error {
	m.mu.Lock()
	m.dir = dir
	m.mu.Unlock()
	return m.saveAppConfig(appConfig{VaultPath: dir})
}

// Init scaffolds (or adopts) a vault at path and returns its status.
// remote is optional ("origin").
func (m *Manager) Init(ctx context.Context, g git.Gitter, path, name, remote string) (*model.VaultStatus, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("vault path must be absolute")
	}
	path = filepath.Clean(path)
	if strings.HasPrefix(path, xdg.ConfigHome) {
		return nil, errors.New("vault path must not be inside the app config directory")
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	if err := git.Init(ctx, g, path); err != nil {
		return nil, err
	}
	if err := git.EnsureIdentity(ctx, g, path); err != nil {
		return nil, err
	}
	// Scaffold .rapi/config.yml + sample collection when the vault is empty.
	if _, err := store.LoadVaultConfig(path); err != nil {
		if err := scaffold(path, name); err != nil {
			return nil, err
		}
	}
	if remote != "" {
		if err := git.RemoteAdd(ctx, g, path, "origin", remote); err != nil {
			return nil, err
		}
	}
	// Initial commit when the repo has no history yet.
	if !git.HasCommits(ctx, g, path) {
		if _, err := git.Commit(ctx, g, path, "rAPI: initialize vault"); err != nil {
			return nil, err
		}
	}
	if err := m.SetDir(path); err != nil {
		return nil, err
	}
	return m.Status(ctx, g), nil
}

// Open validates and activates an existing vault.
func (m *Manager) Open(ctx context.Context, g git.Gitter, path string) (*model.VaultStatus, error) {
	path = filepath.Clean(path)
	if !git.IsRepo(ctx, g, path) {
		return nil, fmt.Errorf("%s is not a git repository", path)
	}
	if _, err := store.LoadVaultConfig(path); err != nil {
		return nil, fmt.Errorf("not a rAPI vault (missing or invalid .rapi/config.yml): %w", err)
	}
	if err := m.SetDir(path); err != nil {
		return nil, err
	}
	return m.Status(ctx, g), nil
}

// Status summarises the configured vault.
func (m *Manager) Status(ctx context.Context, g git.Gitter) *model.VaultStatus {
	st := &model.VaultStatus{}
	path := m.Dir()
	if path == "" {
		path = m.loadAppConfig().VaultPath
		st.Configured = path != ""
	} else {
		st.Configured = true
	}
	if path == "" {
		return st
	}
	st.Path = path
	if cfg, err := store.LoadVaultConfig(path); err == nil {
		st.Name = cfg.Name
		st.Valid = git.IsRepo(ctx, g, path)
	} else {
		st.Valid = false
	}
	st.Remotes = git.Remotes(ctx, g, path)
	if url := git.RemoteURL(ctx, g, path, "origin"); url != "" {
		st.HasRemote = true
		st.RemoteURL = url
	}
	return st
}

// AddRemote registers origin.
func (m *Manager) AddRemote(ctx context.Context, g git.Gitter, url string) error {
	dir := m.Dir()
	if dir == "" {
		return ErrNoVault
	}
	if git.RemoteURL(ctx, g, dir, "origin") != "" {
		return errors.New("remote 'origin' already exists (remove it first)")
	}
	return git.RemoteAdd(ctx, g, dir, "origin", url)
}

// RemoveRemote removes a remote by name.
func (m *Manager) RemoveRemote(ctx context.Context, g git.Gitter, name string) error {
	dir := m.Dir()
	if dir == "" {
		return ErrNoVault
	}
	return git.RemoteRemove(ctx, g, dir, name)
}

// scaffold writes the initial vault content: .rapi/config.yml + sample collection.
func scaffold(path, name string) error {
	if strings.TrimSpace(name) == "" {
		name = "My Vault"
	}
	if err := store.SaveVaultConfig(path, &model.VaultConfig{
		FormatVersion: model.FormatVersion,
		Name:          name,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return err
	}
	return writeSampleCollection(path)
}

// writeSampleCollection drops a small dogfooding collection into the vault.
func writeSampleCollection(vaultPath string) error {
	dir := filepath.Join(vaultPath, "sample-api")
	files := map[string]string{
		"opencollection.yml": `opencollection: "1.0.0"
info:
  name: Sample API
  summary: Sample collection created by the rAPI onboarding wizard
  version: "1.0.0"
config:
  environments:
    - local
    - production
request:
  headers:
    - name: Accept
      value: application/json
  variables:
    - name: baseUrl
      value: https://httpbin.org
  auth: inherit
docs: |
  # Sample API

  A starter collection. Delete it or edit the files directly —
  this vault is a plain git repo.
`,
		"environments/local.yml": `name: local
color: "#1a7f37"
description: Local development
variables:
  - name: baseUrl
    value: https://httpbin.org
`,
		"environments/production.yml": `name: production
color: "#9b111e"
description: Production
variables:
  - name: baseUrl
    value: https://httpbin.org
`,
		"httpbin/folder.yml": `info:
  name: HTTP Bin
  type: folder
  seq: 1
docs: |
  ## HTTP Bin

  Echo endpoints for a first smoke test.
request:
  variables:
    - name: resource
      value: get
`,
		"httpbin/get-anything.yml": `info:
  name: Get Anything
  type: http
  seq: 1
  description: Echo any request
http:
  method: GET
  url: "{{baseUrl}}/{{resource}}"
  params:
    - name: source
      value: rapi
      type: query
  auth:
    type: inherit
settings:
  encodeUrl: true
  timeout: 30
  followRedirects: true
  maxRedirects: 5
docs: |
  ## Get Anything

  Returns whatever you sent. Uses ` + "`{{baseUrl}}`" + ` from the active environment.
`,
	}
	for rel, content := range files {
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
