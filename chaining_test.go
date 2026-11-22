package main

// Chaining test (P0 unit A): two requests run through RunService with the
// SAME RunScope. Request A's post-script extracts a token from the response
// body; request B must send that token (scope.Vars chain) instead of the
// stale value defined in the environment — proving scope.Vars is the
// highest-precedence layer and that post-script vars flow across requests.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/adrg/xdg"
	"github.com/friesipayung/rapi/internal/runner"
	"github.com/friesipayung/rapi/internal/vault"
)

// newChainVault writes a minimal two-request collection into a temp vault
// and returns the manager plus the two request ids.
func newChainVault(t *testing.T, baseURL string) (*vault.Manager, string, string) {
	t.Helper()

	// keep vault.New() away from the real ~/.config/rapi/app.json
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg-config"))
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	root := t.TempDir()
	files := map[string]string{
		".rapi/config.yml": "formatVersion: \"1\"\nname: Test Vault\n",
		"chain/opencollection.yml": `opencollection: "1.0.0"
info:
  name: Chain
`,
		// env defines a stale `token`: if chaining/precedence is broken,
		// request B sends this instead of the value extracted by A.
		"chain/environments/local.yml": fmt.Sprintf(`name: local
variables:
  - name: baseUrl
    value: %s
  - name: token
    value: stale-from-env
`, baseURL),
		"chain/login.yml": `info:
  name: Login
  type: http
http:
  method: POST
  url: "{{baseUrl}}/login"
runtime:
  scripts:
    - type: post
      content: |
        console.log("login ok", rapi.response.body.token);
        rapi.vars.set("token", rapi.response.body.token);
`,
		"chain/use-token.yml": `info:
  name: Use Token
  type: http
http:
  method: GET
  url: "{{baseUrl}}/echo"
  headers:
    - name: X-Token
      value: "{{token}}"
`,
	}
	for rel, content := range files {
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := vault.New()
	if err := m.SetDir(root); err != nil {
		t.Fatal(err)
	}
	return m, "chain/login", "chain/use-token"
}

func TestExecuteChainsVarsAcrossScope(t *testing.T) {
	var mu sync.Mutex
	gotToken := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			fmt.Fprint(w, `{"token":"tok-123"}`)
		case "/echo":
			gotToken = r.Header.Get("X-Token")
			fmt.Fprint(w, `{"ok":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr, idA, idB := newChainVault(t, srv.URL)
	rs := NewRunService(mgr)
	ctx := context.Background()
	scope := runner.NewScope()

	// --- request A: post-script extracts the token into the scope ---
	resA, err := rs.executeInScope(ctx, idA, "local", scope)
	if err != nil {
		t.Fatal(err)
	}
	if resA.Error != "" || resA.ScriptError != "" {
		t.Fatalf("request A failed: error=%q scriptError=%q", resA.Error, resA.ScriptError)
	}
	if scope.Vars["token"] != "tok-123" {
		t.Fatalf("after A: scope.Vars[token] = %q, want tok-123", scope.Vars["token"])
	}
	wantA := []string{"login ok tok-123"}
	if !slices.Equal(resA.ConsoleLogs, wantA) {
		t.Fatalf("A console logs = %v, want %v", resA.ConsoleLogs, wantA)
	}

	// --- request B: must carry A's token, not the env fallback ---
	resB, err := rs.executeInScope(ctx, idB, "local", scope)
	if err != nil {
		t.Fatal(err)
	}
	if resB.Error != "" || resB.ScriptError != "" {
		t.Fatalf("request B failed: error=%q scriptError=%q", resB.Error, resB.ScriptError)
	}
	if gotToken != "tok-123" {
		t.Fatalf("request B sent X-Token %q, want tok-123 (chaining broken)", gotToken)
	}

	// console aggregates into the scope but stays per-request on results
	if !slices.Equal(scope.Console, wantA) {
		t.Fatalf("scope.Console = %v, want %v", scope.Console, wantA)
	}
	if len(resB.ConsoleLogs) != 0 {
		t.Fatalf("B console logs must stay per-request, got %v", resB.ConsoleLogs)
	}
}
