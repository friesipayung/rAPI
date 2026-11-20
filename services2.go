package main

// Phase 2 services: request CRUD, environment CRUD, execution, tree mutation.

import (
	"context"
	"fmt"
	"strings"

	"github.com/friesipayung/rapi/internal/engine"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
	"github.com/friesipayung/rapi/internal/vault"
)

// splitID splits "collection/rest/of/path" into collection + rel.
func splitID(id string) (collectionID, rel string, err error) {
	id = strings.TrimPrefix(filepath2slash(id), "/")
	i := strings.Index(id, "/")
	if i <= 0 {
		return "", "", fmt.Errorf("invalid item id %q", id)
	}
	return id[:i], id[i+1:], nil
}

func filepath2slash(s string) string { return strings.ReplaceAll(s, "\\", "/") }

// ---------- TreeService (mutating, phase 2) ----------

// CreateCollection creates a new collection with a default local env.
func (t *TreeService) CreateCollection(ctx context.Context, name string) (*model.TreeNode, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	slug, err := store.CreateCollection(dir, name)
	if err != nil {
		return nil, err
	}
	return store.GetTree(dir, slug)
}

// CreateFolder creates a folder inside a collection (parentRel "" = root).
func (t *TreeService) CreateFolder(ctx context.Context, collectionID, parentRel, name string) (*model.TreeNode, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	if _, err := store.CreateFolder(dir, collectionID, parentRel, name); err != nil {
		return nil, err
	}
	return store.GetTree(dir, collectionID)
}

// CreateRequest creates a new GET request. Returns the full tree (simpler
// than figuring out the parent node client-side).
func (t *TreeService) CreateRequest(ctx context.Context, collectionID, parentRel, name string) (*model.TreeNode, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	if _, err := store.CreateRequest(dir, collectionID, parentRel, name); err != nil {
		return nil, err
	}
	return store.GetTree(dir, collectionID)
}

// DeleteItem removes a request file, folder subtree, or collection.
func (t *TreeService) DeleteItem(ctx context.Context, id string) error {
	dir := t.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return err
	}
	// collection-level (no slash) = delete whole collection
	if rel == "" {
		return store.DeleteCollection(dir, collectionID)
	}
	return store.DeleteTree(dir, collectionID, rel)
}

// RenameItem renames a request file or folder. Returns the new id.
func (t *TreeService) RenameItem(ctx context.Context, id, newName string) (string, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return "", err
	}
	newRel, err := store.Rename(dir, collectionID, rel, newName)
	if err != nil {
		return "", err
	}
	return collectionID + "/" + newRel, nil
}

// ---------- RequestService ----------

// RequestService loads and saves request documents.
type RequestService struct {
	mgr *vault.Manager
}

func NewRequestService(mgr *vault.Manager) *RequestService { return &RequestService{mgr: mgr} }

func (r *RequestService) mustDir() (string, error) {
	dir := r.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	return dir, nil
}

// Get returns the full request document (examples/runtime preserved raw).
func (r *RequestService) Get(ctx context.Context, id string) (*model.RequestDoc, error) {
	dir, err := r.mustDir()
	if err != nil {
		return nil, err
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return nil, err
	}
	return store.LoadRequestDoc(dir, collectionID, rel)
}

// Save persists the request document (auto-commit picks it up).
func (r *RequestService) Save(ctx context.Context, id string, doc *model.RequestDoc) error {
	dir, err := r.mustDir()
	if err != nil {
		return err
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return err
	}
	return store.SaveRequestDoc(dir, collectionID, rel, doc)
}

// GetCollectionDoc returns the collection document (docs tab, defaults).
func (r *RequestService) GetCollectionDoc(ctx context.Context, collectionID string) (*model.CollectionDoc, error) {
	dir, err := r.mustDir()
	if err != nil {
		return nil, err
	}
	return store.LoadCollectionDoc(dir, collectionID)
}

// SaveCollectionDoc updates the collection document.
func (r *RequestService) SaveCollectionDoc(ctx context.Context, collectionID string, doc *model.CollectionDoc) error {
	dir, err := r.mustDir()
	if err != nil {
		return err
	}
	return store.SaveCollectionDoc(dir, collectionID, doc)
}

// GetFolderDoc returns folder.yml (docs + defaults).
func (r *RequestService) GetFolderDoc(ctx context.Context, id string) (*model.FolderDoc, error) {
	dir, err := r.mustDir()
	if err != nil {
		return nil, err
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return nil, err
	}
	return store.LoadFolderDoc(dir, collectionID, rel)
}

// SaveFolderDoc updates folder.yml.
func (r *RequestService) SaveFolderDoc(ctx context.Context, id string, doc *model.FolderDoc) error {
	dir, err := r.mustDir()
	if err != nil {
		return err
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return err
	}
	return store.SaveFolderDoc(dir, collectionID, rel, doc)
}

// ---------- EnvService ----------

// EnvService manages environments + the active-env UI state.
type EnvService struct {
	mgr *vault.Manager
}

func NewEnvService(mgr *vault.Manager) *EnvService { return &EnvService{mgr: mgr} }

// List returns all environments of a collection.
func (e *EnvService) List(ctx context.Context, collectionID string) ([]*model.EnvironmentDoc, error) {
	dir := e.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	return store.ListEnvironments(dir, collectionID)
}

// Save writes an environment (create or update).
func (e *EnvService) Save(ctx context.Context, collectionID string, env *model.EnvironmentDoc) error {
	dir := e.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	return store.SaveEnvironment(dir, collectionID, env)
}

// Delete removes an environment.
func (e *EnvService) Delete(ctx context.Context, collectionID, name string) error {
	dir := e.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	return store.DeleteEnvironment(dir, collectionID, name)
}

// SetActive persists the active env per collection in .rapi/ui-state.yml.
func (e *EnvService) SetActive(ctx context.Context, collectionID, envName string) error {
	dir := e.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	st, _ := store.LoadUIState(dir)
	st.ActiveEnvs[collectionID] = envName
	return store.SaveUIState(dir, st)
}

// ActiveState returns the full UI state (active envs + theme).
func (e *EnvService) ActiveState(ctx context.Context) (*model.UIState, error) {
	dir := e.mgr.Dir()
	if dir == "" {
		return &model.UIState{ActiveEnvs: map[string]string{}}, nil
	}
	return store.LoadUIState(dir)
}

// ---------- RunService ----------

// RunService executes requests with the env chain applied.
type RunService struct {
	mgr *vault.Manager
}

func NewRunService(mgr *vault.Manager) *RunService { return &RunService{mgr: mgr} }

// Execute runs a request: merges collection/folder defaults + active env
// vars, interpolates, and performs the HTTP call.
func (s *RunService) Execute(ctx context.Context, id, envName string) (*model.RunResult, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return nil, err
	}
	doc, err := store.LoadRequestDoc(dir, collectionID, rel)
	if err != nil {
		return nil, err
	}
	col, err := store.LoadCollectionDoc(dir, collectionID)
	if err != nil {
		return nil, err
	}

	// env selection: explicit param > ui-state > none
	st, _ := store.LoadUIState(dir)
	if envName == "" {
		envName = st.ActiveEnvs[collectionID]
	}
	envs, _ := store.ListEnvironments(dir, collectionID)
	lookup := func(name string) *model.EnvironmentDoc {
		for _, e := range envs {
			if strings.EqualFold(e.Name, name) {
				return e
			}
		}
		return nil
	}
	env := lookup(envName)

	// nearest folder.yml up the chain
	folder := nearestFolder(dir, collectionID, rel)

	eff, settings, _ := engine.Effective(col, folder, doc, env, lookup)
	exec := &engine.Executor{}
	return exec.Run(eff, settings, envName), nil
}

// nearestFolder finds the closest ancestor directory holding folder.yml.
func nearestFolder(vaultPath, collectionID, rel string) *model.FolderDoc {
	parts := strings.Split(rel, "/")
	for i := len(parts) - 1; i >= 1; i-- {
		dirRel := strings.Join(parts[:i], "/")
		doc, err := store.LoadFolderDoc(vaultPath, collectionID, dirRel)
		if err == nil {
			return doc
		}
	}
	return nil
}
