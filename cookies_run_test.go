package main

// Cookie jar wiring test (P0 unit F, task 6.3): RunService.executeInScope
// must attach the persistent app jar by default (cookie set by response 1 is
// replayed on request 2, even by a FRESH RunService — the "restart" case),
// while settings.sendCookies: false opts the request out entirely (nothing
// sent, nothing stored). The store itself lives outside the vault.

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adrg/xdg"
	"github.com/friesipayung/rapi/internal/cookies"
	"github.com/friesipayung/rapi/internal/vault"
)

// newCookieVault writes a minimal one-request collection into a temp vault
// and returns the manager plus the request id.
func newCookieVault(t *testing.T, baseURL, extraSettings string) (*vault.Manager, string) {
	t.Helper()

	// keep vault.New() away from the real ~/.config/rapi/app.json and the
	// cookie store away from the real XDG state dir
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "xdg-state"))
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	root := t.TempDir()
	doc := fmt.Sprintf(`info:
  name: Cookie
  type: http
http:
  method: GET
  url: "%s/x"
%s
`, baseURL, extraSettings)
	files := map[string]string{
		".rapi/config.yml":                 "formatVersion: \"1\"\nname: Test Vault\n",
		"cookiejar/opencollection.yml":     "opencollection: \"1.0.0\"\ninfo:\n  name: CookieJar\n",
		"cookiejar/environments/local.yml": fmt.Sprintf("name: local\nvariables:\n  - name: baseUrl\n    value: %s\n", baseURL),
		"cookiejar/req.yml":                doc,
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
	return m, "cookiejar/req"
}

// newCookieServer answers "no-cookie" and sets sid=sesame until a request
// actually carries that cookie, then answers "with-cookie".
func newCookieServer(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hasCookie := false
		for _, c := range r.Cookies() {
			if c.Name == "sid" && c.Value == "sesame" {
				hasCookie = true
			}
		}
		// headers before the first body write
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "sesame", Path: "/"})
		if hasCookie {
			fmt.Fprint(w, "with-cookie")
		} else {
			fmt.Fprint(w, "no-cookie")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func body(t *testing.T, data string) string {
	t.Helper()
	if strings.TrimSpace(data) == "" {
		t.Fatalf("empty response body")
	}
	return data
}

func TestRunServiceWiresCookieJarByDefault(t *testing.T) {
	srv := newCookieServer(t)
	mgr, id := newCookieVault(t, srv.URL, "")
	ctx := context.Background()

	rs := NewRunService(mgr)
	res1, err := rs.Execute(ctx, id, "local")
	if err != nil {
		t.Fatal(err)
	}
	if res1.Error != "" || body(t, res1.Body.Data) != "no-cookie" {
		t.Fatalf("request 1 = %q / %q", res1.Error, res1.Body.Data)
	}
	res2, err := rs.Execute(ctx, id, "local")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Error != "" || res2.Body.Data != "with-cookie" {
		t.Fatalf("request 2 must replay the cookie, got %q / %q", res2.Error, res2.Body.Data)
	}

	// the store sits OUTSIDE the vault and holds the cookie
	views, err := cookies.List(mgr.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Name != "sid" || views[0].Value != "sesame" {
		t.Fatalf("store = %+v", views)
	}

	// restart: a fresh RunService must load the jar from the store and keep
	// replaying the cookie
	rs2 := NewRunService(mgr)
	res3, err := rs2.Execute(ctx, id, "local")
	if err != nil {
		t.Fatal(err)
	}
	if res3.Error != "" || res3.Body.Data != "with-cookie" {
		t.Fatalf("after restart the jar must persist, got %q / %q", res3.Error, res3.Body.Data)
	}
}

func TestRunServiceSendCookiesFalseOptsOut(t *testing.T) {
	srv := newCookieServer(t)
	mgr, id := newCookieVault(t, srv.URL, "settings:\n  sendCookies: false\n")
	ctx := context.Background()

	rs := NewRunService(mgr)
	for i := 1; i <= 2; i++ {
		res, err := rs.Execute(ctx, id, "local")
		if err != nil {
			t.Fatal(err)
		}
		if res.Error != "" || res.Body.Data != "no-cookie" {
			t.Fatalf("request %d with sendCookies:false must not carry a cookie, got %q / %q", i, res.Error, res.Body.Data)
		}
	}

	// opting out must not store anything either
	views, err := cookies.List(mgr.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Fatalf("opted-out request stored cookies: %+v", views)
	}
}

func TestCookieServiceListTruncatesAndClearDropsJar(t *testing.T) {
	srv := newCookieServer(t)
	mgr, id := newCookieVault(t, srv.URL, "")
	ctx := context.Background()

	rs := NewRunService(mgr)
	if _, err := rs.Execute(ctx, id, "local"); err != nil {
		t.Fatal(err)
	}
	if _, err := rs.Execute(ctx, id, "local"); err != nil {
		t.Fatal(err)
	}

	svc := NewCookieService(mgr, rs)
	views, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("List = %+v", views)
	}
	// "sesame" is short enough to stay whole; verify truncation with a long
	// value by writing one straight into the jar.
	long := "super-secret-cookie-value-0123456789"
	jar, err := cookies.Load(mgr.Dir())
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(mustURL(t, srv.URL+"/"), []*http.Cookie{{Name: "big", Value: long, Path: "/"}})
	if err := jar.Save(mgr.Dir()); err != nil {
		t.Fatal(err)
	}
	views, err = svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if v.Value == long {
			t.Fatalf("List must truncate long cookie values, got %q", v.Value)
		}
	}

	// Clear must empty the store AND drop the in-memory jar: the next
	// Execute must not re-save the cleared cookies.
	if err := svc.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	views, err = svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Fatalf("store not cleared: %+v", views)
	}
	res, err := rs.Execute(ctx, id, "local")
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" || res.Body.Data != "no-cookie" {
		t.Fatalf("after Clear the request must start cookieless, got %q", res.Body.Data)
	}
	views, _ = cookies.List(mgr.Dir())
	if len(views) != 1 { // only the fresh sid from this request
		t.Fatalf("after clear+execute the store must hold only the new cookie, got %+v", views)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// stateFiles lists every file under the (test-redirected) XDG_STATE_HOME.
// A missing state root counts as zero files.
func stateFiles(t *testing.T, stateDir string) []string {
	t.Helper()
	if _, err := os.Stat(stateDir); err != nil {
		return nil
	}
	var files []string
	err := filepath.Walk(stateDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestCLIRunServiceNeverTouchesCookieStore pins the Unit F FINAL RULING at
// the RunService seam: exactly like run.go (construct, then
// SetCookiesEnabled(false)), a request that receives Set-Cookie must neither
// replay nor persist it, and no cookie store file may appear anywhere under
// XDG_STATE_HOME.
func TestCLIRunServiceNeverTouchesCookieStore(t *testing.T) {
	srv := newCookieServer(t)
	mgr, id := newCookieVault(t, srv.URL, "")
	ctx := context.Background()

	// Mirror run.go: construct, then hard-off before running anything.
	rs := NewRunService(mgr)
	rs.SetCookiesEnabled(false)

	for i := 1; i <= 2; i++ {
		res, err := rs.Execute(ctx, id, "local")
		if err != nil {
			t.Fatal(err)
		}
		if res.Error != "" || res.Body.Data != "no-cookie" {
			t.Fatalf("CLI request %d must not carry a cookie, got %q / %q", i, res.Error, res.Body.Data)
		}
	}

	if files := stateFiles(t, os.Getenv("XDG_STATE_HOME")); len(files) != 0 {
		t.Fatalf("CLI-constructed RunService created cookie store files: %v", files)
	}
}

// TestRunCLINeverCreatesCookieStore pins the ruling at the real CLI entry
// point: `rapi run` over a collection whose requests receive Set-Cookie must
// exit 0 and leave the XDG state tree empty (--cookies stays a reserved
// no-op, there is no knob that turns the jar on for the CLI).
func TestRunCLINeverCreatesCookieStore(t *testing.T) {
	srv := newCookieServer(t)
	mgr, _ := newCookieVault(t, srv.URL, "")
	vaultDir := mgr.Dir()

	var out bytes.Buffer
	code := runCLIOut(&out, []string{"cookiejar", "--vault", vaultDir, "--env", "local"})
	if code != runExitOK {
		t.Fatalf("rapi run exit = %d, output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "1 passed") {
		t.Fatalf("unexpected run summary:\n%s", out.String())
	}
	if files := stateFiles(t, os.Getenv("XDG_STATE_HOME")); len(files) != 0 {
		t.Fatalf("rapi run created cookie store files: %v", files)
	}
}
