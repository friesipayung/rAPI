// Package cookies holds the persistent cookie jar used by request execution.
//
// Storage lives OUTSIDE the git vault on purpose: cookies are credentials and
// must never be committed. The store file is
// $XDG_STATE_HOME/rapi/cookies/<sha256(abs vaultPath)>.json and holds
//
//	{"version":1,"entries":[{"host":"example.com",...},...]}
//
// Design note (documented deviation from the plan text): the plan asked for
// persistence via jar.Serialize/Deserialize, but net/http/cookiejar (go1.26)
// exposes no serialization at all — only New/Cookies/SetCookies — and its
// entries are unexported. The unit brief authorizes wrapping the stdlib jar
// with SetCookies/Cookies hooks that record seen cookies in a slice
// "persisted alongside". Because there is no gob to persist beside them, the
// recorded entries ARE the persistence: Save writes them, Load rebuilds a
// fresh stdlib cookiejar by re-issuing SetCookies with the recorded
// attributes (domain attribute, path, secure, httpOnly, expires), which
// reproduces the jar's observable behavior.
package cookies

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// fileFormat is the on-disk document (version 1).
type fileFormat struct {
	Version int            `json:"version"`
	Entries []storedCookie `json:"entries,omitempty"`
}

// storedCookie is one observed cookie as persisted. Expires is RFC3339 in
// UTC, empty for session cookies. Domain is the raw Domain attribute (empty
// for host-only cookies, possibly with a leading dot); Host is the hostname
// the cookie was received from and doubles as the display domain.
type storedCookie struct {
	Host     string `json:"host"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Secure   bool   `json:"secure,omitempty"`
	HttpOnly bool   `json:"httpOnly,omitempty"`
	Expires  string `json:"expires,omitempty"`
}

// CookieView is the UI projection of a stored cookie (values are truncated
// by the service layer before they reach the frontend — they are
// credentials).
type CookieView struct {
	Domain  string `json:"domain"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Expires string `json:"expires"`
}

// Jar is an http.CookieJar that wraps the stdlib cookiejar and records every
// cookie it is asked to store, so the UI can inspect the jar's content.
type Jar struct {
	*cookiejar.Jar

	mu      sync.Mutex
	entries []storedCookie
}

// New returns an empty recording jar.
func New() *Jar {
	jar, err := cookiejar.New(nil)
	if err != nil { // nil options: never happens
		panic("cookies: cookiejar.New with nil options failed: " + err.Error())
	}
	return &Jar{Jar: jar}
}

// Load reads the cookie store for the given vault and rebuilds a jar from
// the recorded cookies. A missing (or empty) store returns a fresh empty jar
// with a nil error; a corrupt store is reported as an error so callers can
// decide (RunService falls back to running without cookies rather than
// overwriting the file).
func Load(vaultPath string) (*Jar, error) {
	raw, err := os.ReadFile(statePath(vaultPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return New(), nil
	}
	var file fileFormat
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("cookies: corrupt store: %w", err)
	}
	jar := New()
	now := time.Now()
	// Re-insert in recorded order so the jar's creation-time cookie order is
	// stable across restarts. Expired entries are skipped (the jar would
	// drop them at read time anyway).
	for _, e := range file.Entries {
		c := &http.Cookie{
			Name:     e.Name,
			Value:    e.Value,
			Path:     e.Path,
			Domain:   e.Domain,
			Secure:   e.Secure,
			HttpOnly: e.HttpOnly,
		}
		if e.Expires != "" {
			t, err := time.Parse(time.RFC3339, e.Expires)
			if err != nil {
				continue // unreadable expiry: skip rather than fail the load
			}
			if t.Before(now) {
				continue // expired while the app was away
			}
			c.Expires = t
		}
		scheme := "https"
		if !e.Secure {
			scheme = "http" // keep non-secure cookies non-secure
		}
		u := &url.URL{Scheme: scheme, Host: e.Host, Path: e.Path}
		jar.SetCookies(u, []*http.Cookie{c}) // records the entry again
	}
	return jar, nil
}

// Save atomically persists the recorded cookies (temp file + rename in the
// store directory). The directory is created with 0700 and the file written
// with 0600 — the content is credentials.
func (j *Jar) Save(vaultPath string) error {
	j.mu.Lock()
	file := fileFormat{Version: 1, Entries: append([]storedCookie(nil), j.entries...)}
	j.mu.Unlock()

	blob, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("cookies: encode: %w", err)
	}

	path := statePath(vaultPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cookies-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(blob); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Clear removes the persisted store for the given vault (missing file is
// fine). The in-memory jar of a running app is unaffected; the CookieService
// resets it separately.
func Clear(vaultPath string) error {
	err := os.Remove(statePath(vaultPath))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns the cookies recorded in the stored file for the given vault,
// sorted by domain then name. A missing store yields an empty list.
func List(vaultPath string) ([]CookieView, error) {
	raw, err := os.ReadFile(statePath(vaultPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []CookieView{}, nil
		}
		return nil, err
	}
	var file fileFormat
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("cookies: corrupt store: %w", err)
	}
	out := make([]CookieView, 0, len(file.Entries))
	for _, e := range file.Entries {
		domain := e.Host
		if e.Domain != "" {
			domain = strings.TrimPrefix(e.Domain, ".")
		}
		out = append(out, CookieView{Domain: domain, Name: e.Name, Value: e.Value, Expires: e.Expires})
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].Domain != out[k].Domain {
			return out[i].Domain < out[k].Domain
		}
		return out[i].Name < out[k].Name
	})
	return out, nil
}

// SetCookies stores the cookies via the wrapped jar and records them for the
// viewer. Cookies the stdlib rejects are still recorded (best effort — the
// viewer shows what the server sent). Deletion hints (Max-Age < 0 or an
// already-expired Expires) drop the matching recorded entry.
func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.Jar.SetCookies(u, cookies)

	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		host := u.Hostname()
		path := c.Path
		if path == "" {
			path = "/"
		}
		if c.MaxAge < 0 { // RFC 6265 deletion hint (net/http normalizes Max-Age<=0 to -1)
			j.remove(host, path, c.Name)
			continue
		}
		e := storedCookie{
			Host:     host,
			Domain:   c.Domain,
			Path:     path,
			Name:     c.Name,
			Value:    c.Value,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
		}
		if c.MaxAge > 0 {
			e.Expires = now.Add(time.Duration(c.MaxAge) * time.Second).UTC().Format(time.RFC3339)
		} else if !c.Expires.IsZero() {
			if c.Expires.Before(now) { // already expired: nothing to keep
				j.remove(host, path, c.Name)
				continue
			}
			e.Expires = c.Expires.UTC().Format(time.RFC3339)
		}
		j.upsert(e)
	}
}

// Cookies returns the cookies to send for u (the wrapped jar decides).
func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	return j.Jar.Cookies(u)
}

// Entries returns a snapshot of the recorded cookies for the UI, sorted by
// domain then name.
func (j *Jar) Entries() []CookieView {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]CookieView, 0, len(j.entries))
	for _, e := range j.entries {
		domain := e.Host
		if e.Domain != "" {
			domain = strings.TrimPrefix(e.Domain, ".")
		}
		out = append(out, CookieView{Domain: domain, Name: e.Name, Value: e.Value, Expires: e.Expires})
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].Domain != out[k].Domain {
			return out[i].Domain < out[k].Domain
		}
		return out[i].Name < out[k].Name
	})
	return out
}

// upsert replaces the (host, path, name) entry or appends it. Callers hold
// j.mu.
func (j *Jar) upsert(e storedCookie) {
	for i := range j.entries {
		if j.entries[i].Host == e.Host && j.entries[i].Path == e.Path && j.entries[i].Name == e.Name {
			j.entries[i] = e
			return
		}
	}
	j.entries = append(j.entries, e)
}

// remove deletes the (host, path, name) entry if present. Callers hold j.mu.
func (j *Jar) remove(host, path, name string) {
	for i := range j.entries {
		if j.entries[i].Host == host && j.entries[i].Path == path && j.entries[i].Name == name {
			j.entries = append(j.entries[:i], j.entries[i+1:]...)
			return
		}
	}
}

// statePath returns the store path for a vault:
// $XDG_STATE_HOME/rapi/cookies/<sha256(abs vaultPath)>.json with
// XDG_STATE_HOME defaulting to ~/.local/state. The vault path is resolved to
// an absolute path before hashing so relative and absolute invocations of
// the same vault share one store.
func statePath(vaultPath string) string {
	if abs, err := filepath.Abs(vaultPath); err == nil {
		vaultPath = abs
	}
	sum := sha256.Sum256([]byte(vaultPath))
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "rapi", "cookies", hex.EncodeToString(sum[:])+".json")
}
