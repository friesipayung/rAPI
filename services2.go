package main

// Phase 2 services: request CRUD, environment CRUD, execution, tree mutation.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/friesipayung/rapi/internal/cookies"
	"github.com/friesipayung/rapi/internal/curl"
	"github.com/friesipayung/rapi/internal/engine"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/runner"
	"github.com/friesipayung/rapi/internal/secrets"
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

// CreateFromCommand pastes a curl/wget command line as a new request.
// Returns the new node id ("<collectionID>/<rel>").
func (t *TreeService) CreateFromCommand(ctx context.Context, collectionID, parentRel, name, cmdline string) (string, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	req, err := curl.Parse(cmdline)
	if err != nil {
		return "", err
	}
	if name == "" {
		name = DeriveRequestName(req.URL)
	}
	rel, err := store.CreateRequestWithData(dir, collectionID, parentRel, name, req)
	if err != nil {
		return "", err
	}
	return collectionID + "/" + rel, nil
}

// DeriveRequestName builds a readable default name from a URL.
func DeriveRequestName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "Pasted request"
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return u.Host
	}
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]
	if last == "" {
		return u.Host + "/" + strings.Join(segs[:len(segs)-1], "/")
	}
	return u.Host + "/" + path
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

// DuplicateItem copies a request, folder or whole collection (append
// " (copy)" to the name). Returns the new item id.
func (t *TreeService) DuplicateItem(ctx context.Context, id string) (string, error) {
	dir := t.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	collectionID, rel, err := splitID(id)
	if err != nil {
		return "", err
	}
	if rel == "" {
		slug, err := store.DuplicateCollection(dir, collectionID)
		if err != nil {
			return "", err
		}
		return slug, nil
	}
	// folder = directory at rel; request = rel + ".yml"
	if fi, statErr := os.Stat(filepath.Join(dir, collectionID, rel)); statErr == nil && fi.IsDir() {
		newRel, err := store.DuplicateFolder(dir, collectionID, rel)
		if err != nil {
			return "", err
		}
		return collectionID + "/" + newRel, nil
	}
	newRel, err := store.DuplicateRequest(dir, collectionID, rel)
	if err != nil {
		return "", err
	}
	return collectionID + "/" + newRel, nil
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

// SetOpenTabs persists the open request tabs + active tab in
// .rapi/ui-state.yml, next to the active-env state.
func (e *EnvService) SetOpenTabs(ctx context.Context, ids []string, active string) error {
	dir := e.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	st, _ := store.LoadUIState(dir)
	st.OpenTabs = ids
	st.ActiveTab = active
	return store.SaveUIState(dir, st)
}

// ---------- secrets (OS keyring, phase 4) ----------

// SetSecret stores a variable value in the OS keyring. The value never
// enters the git vault; the YAML variable keeps an empty value + secret flag.
func (e *EnvService) SetSecret(ctx context.Context, collectionID, envName, name, value string) error {
	if collectionID == "" || envName == "" {
		return fmt.Errorf("collection and environment are required")
	}
	return secrets.Set(collectionID, envName, name, value)
}

// HasSecret reports whether the keyring holds a value for the variable.
func (e *EnvService) HasSecret(ctx context.Context, collectionID, envName, name string) (bool, error) {
	return secrets.Has(collectionID, envName, name), nil
}

// DeleteSecret removes the keyring entry for a variable.
func (e *EnvService) DeleteSecret(ctx context.Context, collectionID, envName, name string) error {
	return secrets.Delete(collectionID, envName, name)
}

// GetSecretValue is NOT exposed: secret values are write-only for the UI and
// read-only for the engine at execution time.

// ---------- RunService ----------

// RunService executes requests with the env chain applied. It owns the app
// cookie jar: one in-memory jar, lazily loaded from the persistent cookie
// store (internal/cookies — stored OUTSIDE the vault, cookies are
// credentials) and saved back after every executed request.
type RunService struct {
	mgr *vault.Manager

	// cookiesOn is the app-level master switch for the jar. The GUI keeps it
	// on (default from NewRunService); the CLI (`rapi run`) turns it OFF —
	// per the Unit F ruling the CLI never loads, sends or persists cookies.
	// It is read in executeInScope without a lock because it is only flipped
	// once at construction time, before any request runs.
	cookiesOn bool

	jarMu    sync.Mutex
	jar      *cookies.Jar
	jarVault string // vault path the in-memory jar was loaded for
}

func NewRunService(mgr *vault.Manager) *RunService { return &RunService{mgr: mgr, cookiesOn: true} }

// SetCookiesEnabled flips the app-level cookie jar switch (default on). The
// CLI calls it with false right after construction, before running anything:
// no cookie store is read, no jar is attached, nothing is persisted.
func (s *RunService) SetCookiesEnabled(on bool) { s.cookiesOn = on }

// appJar returns the in-memory cookie jar for the given vault, loading it
// from the store on first use (and again when the active vault changes).
// A broken store must not break execution: on load error the jar is off for
// requests and nothing is overwritten.
func (s *RunService) appJar(vaultPath string) *cookies.Jar {
	s.jarMu.Lock()
	defer s.jarMu.Unlock()
	if s.jar == nil || s.jarVault != vaultPath {
		j, err := cookies.Load(vaultPath)
		if err != nil || j == nil {
			s.jar, s.jarVault = nil, ""
			return nil
		}
		s.jar, s.jarVault = j, vaultPath
	}
	return s.jar
}

// saveJar persists the in-memory jar for the given vault (best effort — a
// failed write only means cookies may not survive a restart).
func (s *RunService) saveJar(vaultPath string, jar *cookies.Jar) {
	if jar == nil {
		return
	}
	_ = jar.Save(vaultPath)
}

// dropJar forgets the in-memory jar so the next request starts from the
// (just cleared) store. Used by CookieService.Clear.
func (s *RunService) dropJar() {
	s.jarMu.Lock()
	defer s.jarMu.Unlock()
	s.jar, s.jarVault = nil, ""
}

// Execute runs a request: merges collection/folder defaults + active env
// vars, interpolates, and performs the HTTP call. Each call uses a fresh
// run scope — single-shot execution, no chaining between calls.
func (s *RunService) Execute(ctx context.Context, id, envName string) (*model.RunResult, error) {
	scope := runner.NewScope()
	return s.executeInScope(ctx, id, envName, scope)
}

// executeInScope runs one request inside a shared run scope: variables set
// by earlier requests of the run (scope.Vars) take the highest merge
// precedence, script-set variables are written back into the scope so the
// next request can consume them, and console lines aggregate into
// scope.Console. The collection runner drives this directly.
func (s *RunService) executeInScope(ctx context.Context, id, envName string, scope *runner.RunScope) (*model.RunResult, error) {
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

	// variable scope: run scope (chaining, highest) > env > folder >
	// collection; pre-scripts below can still override per request
	vars := engine.MergeVars(col, folder, env, lookup)
	for k, v := range scope.Vars {
		vars[k] = v
	}

	// secret variables: values live in the OS keyring, not in YAML
	var baseEnv *model.EnvironmentDoc
	if env != nil && env.Extends != "" {
		baseEnv = lookup(env.Extends)
	}
	injectSecrets(vars, collectionID, baseEnv, env)

	// pre-scripts may set runtime variables (highest precedence)
	rt := doc.RuntimeAsStruct()
	console := []string{}
	var scriptErrs []string
	for _, sc := range rt.Scripts {
		if sc.Type != model.ScriptPre {
			continue
		}
		out := engine.RunScript(sc, &doc.Http, nil, vars)
		console = append(console, out.Console...)
		if out.Error != "" {
			scriptErrs = append(scriptErrs, "pre: "+out.Error)
			continue
		}
		for k, v := range out.Vars {
			vars[k] = v
			scope.Vars[k] = v // chaining: visible to the next request
		}
	}

	eff, settings := engine.EffectiveWithVars(col, folder, doc, vars)
	e := &engine.Executor{}
	// Cookie jar (P0 task 6.3): UI requests send/receive cookies by default;
	// a request's settings can opt out entirely (sendCookies: false → no jar
	// for that call: nothing sent, nothing stored). The CLI run path is
	// hard-off via SetCookiesEnabled(false) (run.go) — the FINAL RULING of
	// Unit F: the CLI never loads, sends or persists cookies, and --cookies
	// stays a reserved no-op.
	var jar *cookies.Jar
	if s.cookiesOn && settings.CookiesEnabled() {
		jar = s.appJar(dir)
	}
	var opts []engine.RunOpts
	if jar != nil {
		opts = append(opts, engine.RunOpts{Jar: jar})
	}
	res := e.Run(eff, settings, envName, opts...)
	if jar != nil {
		// Persist after every request — cheap (a small JSON document) and it
		// keeps the store fresh for the restart case.
		s.saveJar(dir, jar)
	}
	res.ConsoleLogs = console

	// assertions
	res.Assertions = engine.RunAssertions(rt.Assertions, res)

	// post-scripts (can inspect the response)
	for _, sc := range rt.Scripts {
		if sc.Type != model.ScriptPost {
			continue
		}
		out := engine.RunScript(sc, eff, res, vars)
		res.ConsoleLogs = append(res.ConsoleLogs, out.Console...)
		if out.Error != "" {
			scriptErrs = append(scriptErrs, "post: "+out.Error)
			continue
		}
		// chaining point: post-script vars flow into the next request
		for k, v := range out.Vars {
			scope.Vars[k] = v
		}
	}
	// aggregate console lines into the run scope (res.ConsoleLogs stays
	// per-request)
	scope.Console = append(scope.Console, res.ConsoleLogs...)
	res.ScriptError = strings.Join(scriptErrs, "; ")
	return res, nil
}

// ExecuteInScope satisfies runner.StepExecutor for the collection runner
// loop (internal/runner).
func (s *RunService) ExecuteInScope(ctx context.Context, id, envName string, scope *runner.RunScope) (*model.RunResult, error) {
	return s.executeInScope(ctx, id, envName, scope)
}

var _ runner.StepExecutor = (*RunService)(nil)

// injectSecrets fills empty `secret: true` variable values from the OS
// keyring. Missing keyring entries leave the value empty ({{var}} stays
// unresolved in the request — visible to the user).
func injectSecrets(vars engine.Vars, collectionID string, envs ...*model.EnvironmentDoc) {
	for _, env := range envs {
		if env == nil {
			continue
		}
		for _, v := range env.Variables {
			if !v.Secret || vars[v.Name] != "" {
				continue
			}
			if val, err := secrets.Get(collectionID, env.Name, v.Name); err == nil {
				vars[v.Name] = val
			}
		}
	}
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
