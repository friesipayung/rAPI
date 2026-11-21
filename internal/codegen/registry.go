package codegen

// Registry of targets.

import (
	"fmt"

	"github.com/friesipayung/rapi/internal/model"
)

type emitter func(*Snippet) string

var targets = map[string]Target{}

func register(t Target, e emitter) {
	targets[t.Key] = t
	emitters[t.Key] = e
}

var emitters = map[string]emitter{}

// Targets lists available generators (ordered).
func Targets() []Target {
	var order = []string{"curl", "httpie", "http", "python", "js-fetch", "node-native", "node-axios", "go", "java", "csharp", "php", "ruby", "rust", "swift"}
	var out []Target
	for _, k := range order {
		if t, ok := targets[k]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Generate produces the snippet for a target key.
func Generate(target string, req *model.HttpRequest, settings *model.Settings) (string, error) {
	e, ok := emitters[target]
	if !ok {
		return "", fmt.Errorf("unknown code target %q", target)
	}
	s, err := Build(req, settings)
	if err != nil {
		return "", err
	}
	return e(s), nil
}
