package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

// fakeExec records call order and returns canned outcomes per request id.
type fakeExec struct {
	outcomes map[string]fakeOutcome
	calls    []string
}

type fakeOutcome struct {
	res *model.RunResult
	err error
}

func (f *fakeExec) ExecuteInScope(ctx context.Context, id, envName string, scope *RunScope) (*model.RunResult, error) {
	f.calls = append(f.calls, id)
	o, ok := f.outcomes[id]
	if !ok {
		return &model.RunResult{Status: 200}, nil
	}
	return o.res, o.err
}

func TestRunOrderedExecutionAndOnStep(t *testing.T) {
	exec := &fakeExec{}
	r := &Runner{Exec: exec}
	steps := []Step{
		{RequestID: "col/a", Name: "A"},
		{RequestID: "col/b", Name: "B"},
		{RequestID: "col/c", Name: "C"},
	}
	var seen []string
	scope := NewScope()
	rep := r.Run(context.Background(), steps, "dev", scope, func(sr StepResult) {
		seen = append(seen, sr.RequestID)
		if sr.Result == nil {
			t.Errorf("step %s: onStep got nil Result for an executed step", sr.RequestID)
		}
		if sr.Skipped {
			t.Errorf("step %s: onStep got Skipped=true for an executed step", sr.RequestID)
		}
	})
	if len(exec.calls) != 3 || exec.calls[0] != "col/a" || exec.calls[1] != "col/b" || exec.calls[2] != "col/c" {
		t.Fatalf("executor calls out of order: %v", exec.calls)
	}
	if len(seen) != 3 || seen[0] != "col/a" || seen[1] != "col/b" || seen[2] != "col/c" {
		t.Fatalf("onStep order wrong: %v", seen)
	}
	if rep.EnvUsed != "dev" {
		t.Errorf("EnvUsed = %q, want %q", rep.EnvUsed, "dev")
	}
	if rep.StartedAt.IsZero() {
		t.Error("StartedAt must be set")
	}
	if rep.TotalMs < 0 {
		t.Errorf("TotalMs = %d, must be >= 0", rep.TotalMs)
	}
	if len(rep.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(rep.Steps))
	}
	for i, want := range steps {
		got := rep.Steps[i]
		if got.RequestID != want.RequestID || got.Name != want.Name {
			t.Errorf("Steps[%d] = %s/%s, want %s/%s", i, got.RequestID, got.Name, want.RequestID, want.Name)
		}
		if got.Result == nil || got.Result.Status != 200 {
			t.Errorf("Steps[%d].Result = %+v, want status 200", i, got.Result)
		}
		if got.DurationMs < 0 {
			t.Errorf("Steps[%d].DurationMs = %d, must be >= 0", i, got.DurationMs)
		}
	}
	if rep.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", rep.FailCount)
	}
}

func TestRunContinuesAfterFailure(t *testing.T) {
	exec := &fakeExec{outcomes: map[string]fakeOutcome{
		"col/a": {res: &model.RunResult{Status: 500}},
	}}
	r := &Runner{Exec: exec}
	steps := []Step{
		{RequestID: "col/a", Name: "A"},
		{RequestID: "col/b", Name: "B"},
	}
	var n int
	rep := r.Run(context.Background(), steps, "", NewScope(), func(StepResult) { n++ })
	if len(exec.calls) != 2 {
		t.Fatalf("run must continue after failure, executed %v", exec.calls)
	}
	if n != 2 {
		t.Errorf("onStep called %d times, want 2", n)
	}
	if rep.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", rep.FailCount)
	}
}

func TestRunStopOnFailureSkipsRest(t *testing.T) {
	exec := &fakeExec{outcomes: map[string]fakeOutcome{
		"col/a": {res: &model.RunResult{Status: 500}},
	}}
	r := &Runner{Exec: exec}
	steps := []Step{
		{RequestID: "col/a", Name: "A"},
		{RequestID: "col/b", Name: "B"},
		{RequestID: "col/c", Name: "C"},
	}
	scope := NewScope()
	scope.StopOnFailure = true
	var seen []StepResult
	rep := r.Run(context.Background(), steps, "", scope, func(sr StepResult) { seen = append(seen, sr) })
	if len(exec.calls) != 1 {
		t.Fatalf("after a failure with StopOnFailure only step A must run, got %v", exec.calls)
	}
	if len(seen) != 3 {
		t.Fatalf("onStep must also fire for skipped steps, got %d calls", len(seen))
	}
	if len(rep.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(rep.Steps))
	}
	if rep.Steps[0].Skipped || rep.Steps[0].Result == nil || rep.Steps[0].Result.Status != 500 {
		t.Errorf("Steps[0] must be the failed step, got %+v", rep.Steps[0])
	}
	for i, want := range []Step{{RequestID: "col/b", Name: "B"}, {RequestID: "col/c", Name: "C"}} {
		sr := rep.Steps[i+1]
		if !sr.Skipped || sr.Result != nil {
			t.Errorf("Steps[%d] must be skipped with nil Result, got %+v", i+1, sr)
		}
		if sr.RequestID != want.RequestID || sr.Name != want.Name {
			t.Errorf("skipped step must keep its identity, got %s/%s", sr.RequestID, sr.Name)
		}
	}
	if rep.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1 (skipped steps are not failures)", rep.FailCount)
	}
}

func TestRunGoErrorIsFailedStep(t *testing.T) {
	exec := &fakeExec{outcomes: map[string]fakeOutcome{
		"col/a": {err: errors.New("boom")},
	}}
	r := &Runner{Exec: exec}
	steps := []Step{
		{RequestID: "col/a", Name: "A"},
		{RequestID: "col/b", Name: "B"},
	}
	rep := r.Run(context.Background(), steps, "", NewScope(), nil)
	if len(exec.calls) != 2 {
		t.Fatalf("a Go error must not abort the run, executed %v", exec.calls)
	}
	sr := rep.Steps[0]
	if sr.Result == nil || sr.Result.Error != "boom" {
		t.Errorf("failed step must carry Result.Error = %q, got %+v", "boom", sr.Result)
	}
	if rep.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", rep.FailCount)
	}
}

func TestRunGoErrorWithResultOverwritesError(t *testing.T) {
	exec := &fakeExec{outcomes: map[string]fakeOutcome{
		"col/a": {res: &model.RunResult{Status: 200, Error: "stale"}, err: errors.New("boom")},
	}}
	r := &Runner{Exec: exec}
	rep := r.Run(context.Background(), []Step{{RequestID: "col/a", Name: "A"}}, "", NewScope(), nil)
	if rep.Steps[0].Result == nil || rep.Steps[0].Result.Error != "boom" {
		t.Errorf("Go error must win over Result.Error, got %+v", rep.Steps[0].Result)
	}
	if rep.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", rep.FailCount)
	}
}

func TestRunNoSteps(t *testing.T) {
	r := &Runner{Exec: &fakeExec{}}
	var calls int
	rep := r.Run(context.Background(), nil, "dev", NewScope(), func(StepResult) { calls++ })
	if calls != 0 || len(rep.Steps) != 0 || rep.FailCount != 0 {
		t.Errorf("empty plan must produce an empty report, got %+v", rep)
	}
	if rep.EnvUsed != "dev" {
		t.Errorf("EnvUsed = %q, want %q", rep.EnvUsed, "dev")
	}
}

func TestRunNilScope(t *testing.T) {
	r := &Runner{Exec: &fakeExec{}}
	rep := r.Run(context.Background(), []Step{{RequestID: "col/a", Name: "A"}}, "", nil, nil)
	if rep == nil || len(rep.Steps) != 1 {
		t.Fatalf("nil scope must not panic and still run, got %+v", rep)
	}
}

func TestStepFailed(t *testing.T) {
	tests := []struct {
		name string
		sr   StepResult
		want bool
	}{
		{"skipped is never failed", StepResult{Skipped: true}, false},
		{"skipped with error result is not failed", StepResult{Skipped: true, Result: &model.RunResult{Error: "x"}}, false},
		{"skipped with 500 result is not failed", StepResult{Skipped: true, Result: &model.RunResult{Status: 500}}, false},
		{"nil result without skip is failed", StepResult{}, true},
		{"transport error", StepResult{Result: &model.RunResult{Error: "connection refused"}}, true},
		{"http 400", StepResult{Result: &model.RunResult{Status: 400}}, true},
		{"http 404", StepResult{Result: &model.RunResult{Status: 404}}, true},
		{"http 500", StepResult{Result: &model.RunResult{Status: 500}}, true},
		{"http 399 is not failed", StepResult{Result: &model.RunResult{Status: 399}}, false},
		{"http 200 ok", StepResult{Result: &model.RunResult{Status: 200}}, false},
		{"one assertion failed", StepResult{Result: &model.RunResult{Status: 200, Assertions: []model.AssertionResult{{Pass: true}, {Pass: false}}}}, true},
		{"all assertions pass", StepResult{Result: &model.RunResult{Status: 200, Assertions: []model.AssertionResult{{Pass: true}, {Pass: true}}}}, false},
		{"assertion error counts as failed", StepResult{Result: &model.RunResult{Status: 200, Assertions: []model.AssertionResult{{Error: "empty expression"}}}}, true},
		{"script error", StepResult{Result: &model.RunResult{Status: 200, ScriptError: "post: boom"}}, true},
		{"empty assertion slice", StepResult{Result: &model.RunResult{Status: 200, Assertions: []model.AssertionResult{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stepFailed(tt.sr); got != tt.want {
				t.Errorf("stepFailed(%+v) = %v, want %v", tt.sr, got, tt.want)
			}
		})
	}
}
