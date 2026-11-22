package runner

// Collection run loop: ordered step execution over a shared RunScope with
// per-step reporting, continue/stop-on-failure, and an aggregated Report.
// The per-step executor (RunService in the main package) implements
// StepExecutor; the plan comes from PlanFromTree.

import (
	"context"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

// Step is one planned entry of a collection run: a request node from the
// tree plus its display name.
type Step struct {
	RequestID string
	Name      string
}

// StepResult is the outcome of one step. Result is nil for steps that were
// skipped because StopOnFailure cut the run short before them.
type StepResult struct {
	Step
	Result     *model.RunResult `json:"result"`
	Skipped    bool             `json:"skipped,omitempty"`
	DurationMs int64            `json:"durationMs"`
}

// Report aggregates one full collection run.
type Report struct {
	StartedAt time.Time    `json:"startedAt"`
	TotalMs   int64        `json:"totalMs"`
	EnvUsed   string       `json:"envUsed"`
	Steps     []StepResult `json:"steps"`
	FailCount int          `json:"failCount"`
}

// StepExecutor abstracts per-step execution (implemented by RunService in
// the main package).
type StepExecutor interface {
	ExecuteInScope(ctx context.Context, id, envName string, scope *RunScope) (*model.RunResult, error)
}

// Runner executes a planned sequence of steps against a shared run scope.
type Runner struct {
	Exec StepExecutor
}

// Run executes steps in order and returns a report. onStep (if non-nil) is
// called right after each step completes, including steps skipped by
// StopOnFailure. A step failure does not abort the run unless
// scope.StopOnFailure is set — remaining steps are then reported as
// Skipped. A Go error from ExecuteInScope marks the step failed via
// Result.Error. Skipped steps never count toward FailCount.
func (r *Runner) Run(ctx context.Context, steps []Step, envName string, scope *RunScope, onStep func(StepResult)) *Report {
	if scope == nil {
		scope = NewScope()
	}
	start := time.Now()
	report := &Report{
		StartedAt: start,
		EnvUsed:   envName,
		Steps:     make([]StepResult, 0, len(steps)),
	}
	for i, st := range steps {
		sr := StepResult{Step: st}
		stepStart := time.Now()
		res, err := r.Exec.ExecuteInScope(ctx, st.RequestID, envName, scope)
		sr.DurationMs = time.Since(stepStart).Milliseconds()
		if err != nil {
			if res == nil {
				res = &model.RunResult{}
			}
			res.Error = err.Error()
		}
		sr.Result = res
		report.Steps = append(report.Steps, sr)
		if onStep != nil {
			onStep(sr)
		}
		if scope.StopOnFailure && stepFailed(sr) {
			for _, rest := range steps[i+1:] {
				skipped := StepResult{Step: rest, Skipped: true}
				report.Steps = append(report.Steps, skipped)
				if onStep != nil {
					onStep(skipped)
				}
			}
			break
		}
	}
	report.TotalMs = time.Since(start).Milliseconds()
	for _, sr := range report.Steps {
		if stepFailed(sr) {
			report.FailCount++
		}
	}
	return report
}

// stepFailed reports whether a step counts as failed: no result at all
// (and not skipped), a transport error, an HTTP status >= 400, a script
// error, or any failed assertion. Skipped steps are never failed.
func stepFailed(sr StepResult) bool {
	if sr.Skipped {
		return false
	}
	if sr.Result == nil {
		return true
	}
	if sr.Result.Error != "" || sr.Result.ScriptError != "" || sr.Result.Status >= 400 {
		return true
	}
	for _, a := range sr.Result.Assertions {
		if !a.Pass {
			return true
		}
	}
	return false
}
