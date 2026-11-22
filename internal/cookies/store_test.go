package cookies

// Store tests: XDG redirect, round-trip through Save/Load, Clear, List and
// the missing-file cases. Every test redirects XDG_STATE_HOME to a temp dir
// so the real user state directory is never touched.

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestStatePathHashesAbsVaultPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := t.TempDir()
	rel := filepath.Join(dir, "myvault")
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}

	pRel := statePath(rel)
	pAbs := statePath(abs)
	if pRel != pAbs {
		t.Fatalf("relative and absolute vault paths must hash equal:\n%s\n%s", pRel, pAbs)
	}
	if !strings.Contains(pRel, filepath.Join("rapi", "cookies")) {
		t.Fatalf("expected rapi/cookies in path, got %s", pRel)
	}
	if !strings.HasSuffix(pRel, ".json") {
		t.Fatalf("expected .json suffix, got %s", pRel)
	}
	if len(filepath.Base(pRel)) != 64+5 { // sha256 hex + ".json"
		t.Fatalf("expected sha256 hex filename, got %s", filepath.Base(pRel))
	}
	if strings.Contains(pRel, abs) {
		t.Fatal("vault path must not appear in the store path (privacy)")
	}
}

func TestLoadMissingFileGivesFreshJar(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	jar, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on missing store: %v", err)
	}
	if views := jar.Entries(); len(views) != 0 {
		t.Fatalf("fresh jar must have no entries, got %v", views)
	}
}

func TestRoundTripSaveLoad(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	vault := t.TempDir()

	jar := New()
	expires := time.Now().Add(time.Hour).UTC().Round(time.Second)
	u := testURL(t, "https://example.com/")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "sid", Value: "abc123", Path: "/", Expires: expires},
		{Name: "theme", Value: "dark", Path: "/"}, // session cookie
	})
	if err := jar.Save(vault); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(vault)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The rebuilt jar must still hand the cookie to the same URL.
	got := loaded.Cookies(u)
	var sid string
	for _, c := range got {
		if c.Name == "sid" {
			sid = c.Value
		}
	}
	if sid != "abc123" {
		t.Fatalf("rebuilt jar lost cookie value: %v", got)
	}

	// The viewer entries must round-trip, session cookie without expiry.
	views := loaded.Entries()
	if len(views) != 2 {
		t.Fatalf("expected 2 entries, got %v", views)
	}
	byName := map[string]CookieView{}
	for _, v := range views {
		byName[v.Name] = v
	}
	if byName["sid"].Domain != "example.com" || byName["sid"].Value != "abc123" {
		t.Fatalf("sid entry wrong: %+v", byName["sid"])
	}
	if got, want := byName["sid"].Expires, expires.Format(time.RFC3339); got != want {
		t.Fatalf("sid expires = %q, want %q", got, want)
	}
	if byName["theme"].Expires != "" {
		t.Fatalf("session cookie must have empty expiry, got %q", byName["theme"].Expires)
	}
}

func TestListReadsStoredFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	vault := t.TempDir()

	views, err := List(vault)
	if err != nil || len(views) != 0 {
		t.Fatalf("List on missing store = %v, %v; want empty, nil", views, err)
	}

	jar := New()
	u := testURL(t, "https://api.example.com/v1/")
	jar.SetCookies(u, []*http.Cookie{{Name: "k", Value: "v", Path: "/"}})
	if err := jar.Save(vault); err != nil {
		t.Fatal(err)
	}

	views, err = List(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Name != "k" || views[0].Value != "v" || views[0].Domain != "api.example.com" {
		t.Fatalf("List = %+v", views)
	}
}

func TestClearRemovesStore(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	vault := t.TempDir()

	jar := New()
	jar.SetCookies(testURL(t, "https://example.com/"), []*http.Cookie{{Name: "sid", Value: "x"}})
	if err := jar.Save(vault); err != nil {
		t.Fatal(err)
	}

	// Clear twice: the second call must be a no-op, not an error.
	if err := Clear(vault); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if err := Clear(vault); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
	views, err := List(vault)
	if err != nil {
		t.Fatalf("List after Clear: %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("store not cleared: %+v", views)
	}
}

func TestUpsertAndDelete(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	jar := New()
	u := testURL(t, "https://example.com/")
	jar.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "one"}})
	jar.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "two"}})
	if views := jar.Entries(); len(views) != 1 || views[0].Value != "two" {
		t.Fatalf("upsert failed: %+v", jar.Entries())
	}

	// Deletion hint (Max-Age<0) must drop the recorded entry.
	jar.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "", MaxAge: -1}})
	if views := jar.Entries(); len(views) != 0 {
		t.Fatalf("delete hint ignored: %+v", views)
	}
}
