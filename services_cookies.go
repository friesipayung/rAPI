package main

// CookieService (P0 task 6.4): lists and clears the app cookie jar for the
// current vault. The store lives outside the vault (cookies are credentials
// and must never be committed) — see internal/cookies.

import (
	"context"

	"github.com/friesipayung/rapi/internal/cookies"
	"github.com/friesipayung/rapi/internal/vault"
)

// CookieService exposes the persistent cookie store to the frontend.
type CookieService struct {
	mgr  *vault.Manager
	runs *RunService // to drop the in-memory jar on Clear
}

func NewCookieService(mgr *vault.Manager, runs *RunService) *CookieService {
	return &CookieService{mgr: mgr, runs: runs}
}

// List returns the stored cookies of the current vault with the values
// truncated — cookie values are credentials and must never reach the UI in
// full.
func (s *CookieService) List(ctx context.Context) ([]cookies.CookieView, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	views, err := cookies.List(dir)
	if err != nil {
		return nil, err
	}
	for i := range views {
		views[i].Value = truncateCookieValue(views[i].Value)
	}
	return views, nil
}

// Clear removes the persisted cookie store for the current vault and resets
// the in-memory jar, so the next request starts without cookies.
func (s *CookieService) Clear(ctx context.Context) error {
	dir := s.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	if err := cookies.Clear(dir); err != nil {
		return err
	}
	if s.runs != nil {
		s.runs.dropJar()
	}
	return nil
}

// truncateCookieValue keeps short values whole and mangles longer ones in
// the middle (first 8 + … + last 2), enough to recognize a cookie, not
// enough to reuse it.
func truncateCookieValue(v string) string {
	if len(v) <= 12 {
		return v
	}
	return v[:8] + "…" + v[len(v)-2:]
}
