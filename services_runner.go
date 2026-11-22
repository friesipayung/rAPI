package main

// RunnerService (P0 task 4.1): starts collection runs in the background and
// streams progress to the frontend via Wails events. Each run gets a runID;
// every completed step emits `runner-step` {runID, step}, and the finished
// run emits `runner-done` {runID, report}. The machinery is exactly the CLI's:
// PlanFromTree builds the steps from the same tree the TreeService serves,
// RunService.ExecuteInScope executes each step inside a shared RunScope, and
// internal/runner.Runner drives the loop.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/runner"
	"github.com/friesipayung/rapi/internal/store"
	"github.com/friesipayung/rapi/internal/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// RunnerStepEvent is the payload of the `runner-step` event: one step of a
// run finished. Steps skipped by StopOnFailure are reported too (Result nil,
// Skipped true).
type RunnerStepEvent struct {
	RunID string            `json:"runID"`
	Step  runner.StepResult `json:"step"`
}

// RunnerDoneEvent is the payload of the `runner-done` event: the aggregated
// report of a finished run.
type RunnerDoneEvent struct {
	RunID  string         `json:"runID"`
	Report *runner.Report `json:"report"`
}

func init() {
	application.RegisterEvent[RunnerStepEvent]("runner-step")
	application.RegisterEvent[RunnerDoneEvent]("runner-done")
}

// RunnerEmitter forwards runner events to every window (set in main once the
// app exists — same fan-out pattern as SyncEmitter).
var RunnerEmitter func(name string, payload any)

// RunnerService starts background collection runs for the GUI.
type RunnerService struct {
	mgr *vault.Manager
}

func NewRunnerService(mgr *vault.Manager) *RunnerService { return &RunnerService{mgr: mgr} }

// Start plans the collection/folder subtree identified by scopeID and runs it
// in a background goroutine, returning the runID immediately. Progress is
// emitted as runner-step / runner-done events carrying that runID. envName
// empty means each step uses the collection's active env (the one the
// request-editor EnvPicker persists in ui-state). stopOnFailure skips all
// remaining steps after the first failed one.
func (s *RunnerService) Start(ctx context.Context, scopeID, envName string, stopOnFailure bool) (string, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	scopeID = filepath2slash(strings.TrimPrefix(scopeID, "/"))
	if scopeID == "" {
		return "", fmt.Errorf("empty scope id")
	}
	// scopeID is a collection (no slash) or folder node id; the collection
	// slug is everything before the first slash (same id layout splitID
	// parses for requests).
	collectionID := scopeID
	if i := strings.Index(scopeID, "/"); i > 0 {
		collectionID = scopeID[:i]
	}
	tree, err := store.GetTree(dir, collectionID)
	if err != nil {
		return "", err
	}
	steps := runner.PlanFromTree([]*model.TreeNode{tree}, scopeID)
	if len(steps) == 0 {
		return "", fmt.Errorf("no requests to run under %q", scopeID)
	}
	runID, err := newRunID()
	if err != nil {
		return "", err
	}

	// Wails cancels the bound-method context as soon as Start returns, so
	// the background run detaches from it (values kept, cancellation
	// dropped): a run outlives the call that started it.
	runCtx := context.WithoutCancel(ctx)

	// NewScope (not a bare RunScope literal): Vars must start as an empty
	// non-nil map — script-set variables are chained through it.
	scope := runner.NewScope()
	scope.StopOnFailure = stopOnFailure

	go func() {
		emit := func(name string, payload any) {
			if RunnerEmitter != nil {
				RunnerEmitter(name, payload)
			}
		}
		report := (&runner.Runner{Exec: NewRunService(s.mgr)}).Run(
			runCtx, steps, envName, scope,
			func(sr runner.StepResult) {
				emit("runner-step", RunnerStepEvent{RunID: runID, Step: sr})
			},
		)
		emit("runner-done", RunnerDoneEvent{RunID: runID, Report: report})
	}()
	return runID, nil
}

// newRunID returns 16 hex chars of crypto randomness.
func newRunID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("cannot generate run id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
