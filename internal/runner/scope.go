// Package runner holds run-scoped execution state for the P0 collection
// runner: variables that chain across requests, aggregated console output,
// and a shared cookie jar hook. The per-step executor (RunService in the
// main package) consumes RunScope; the run loop arrives in a later unit.
package runner

import (
	"net/http"

	"github.com/friesipayung/rapi/internal/engine"
)

// RunScope carries the state shared by all requests of one run.
type RunScope struct {
	Vars          engine.Vars    // run-scoped variables (chaining), HIGHEST precedence
	Console       []string       // aggregate console.log lines
	Jar           http.CookieJar // shared between steps (nil = off); consumed in a later unit
	StopOnFailure bool
}

// NewScope returns an empty run scope.
func NewScope() *RunScope { return &RunScope{Vars: engine.Vars{}} }
