package main

// Regression test (P0 review fix wave): RunnerService must reuse the shared
// app RunService — the same instance the bound RunService and CookieService
// hold — so there is exactly ONE in-memory cookie jar. Constructing a fresh
// RunService per run gave the runner its own jar snapshot: a
// CookieService.Clear during a run dropped the shared jar while the run kept
// replaying and re-persisting the cleared cookies (last-writer-wins lost
// updates on the credential store).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/friesipayung/rapi/internal/cookies"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/runner"
	"github.com/friesipayung/rapi/internal/store"
)

func TestRunnerServiceSharesAppRunService(t *testing.T) {
	srv := newCookieServer(t)
	mgr, _ := newCookieVault(t, srv.URL, "")
	ctx := context.Background()

	// A second identical request so one run has two steps; the clear lands
	// between them via the per-step callback.
	doc2 := fmt.Sprintf("info:\n  name: Cookie 2\n  type: http\nhttp:\n  method: GET\n  url: \"%s/x\"\n", srv.URL)
	if err := os.WriteFile(filepath.Join(mgr.Dir(), "cookiejar", "req2.yml"), []byte(doc2), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wire exactly like main.go: ONE RunService handed to both services.
	runSvc := NewRunService(mgr)
	runnerSvc := NewRunnerService(mgr, runSvc)
	cookieSvc := NewCookieService(mgr, runSvc)

	if runnerSvc.runs != runSvc || cookieSvc.runs != runSvc {
		t.Fatal("RunnerService and CookieService must hold the SAME RunService (a second instance keeps a second in-memory jar)")
	}

	tree, err := store.GetTree(mgr.Dir(), "cookiejar")
	if err != nil {
		t.Fatal(err)
	}
	steps := runner.PlanFromTree([]*model.TreeNode{tree}, "cookiejar")
	if len(steps) != 2 {
		t.Fatalf("plan = %+v", steps)
	}

	// Same Exec wiring as RunnerService.Start. Step 1 receives sid=sesame
	// and saves it to the store; Clear mid-run must drop the very jar the
	// run uses, so step 2 goes out cookieless instead of replaying the
	// cleared cookie.
	scope := runner.NewScope()
	cleared := false
	report := (&runner.Runner{Exec: runnerSvc.runs}).Run(ctx, steps, "local", scope, func(sr runner.StepResult) {
		if cleared {
			return
		}
		cleared = true
		if err := cookieSvc.Clear(ctx); err != nil {
			t.Error(err)
		}
	})
	for i, sr := range report.Steps {
		if sr.Skipped || sr.Result == nil || sr.Result.Error != "" {
			t.Fatalf("step %d = %+v", i+1, sr)
		}
		if got := body(t, sr.Result.Body.Data); got != "no-cookie" {
			t.Fatalf("step %d replayed cleared cookies, body = %q", i+1, got)
		}
	}

	// Only the fresh cookie from the post-clear step survives.
	views, err := cookies.List(mgr.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Name != "sid" {
		t.Fatalf("store after clear+run = %+v", views)
	}
}
