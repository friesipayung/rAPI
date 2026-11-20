package model

import "gopkg.in/yaml.v3"

// HttpRequestExample is one OpenCollection `examples[]` entry: an example
// request/response pair stored natively in the request file.
type HttpRequestExample struct {
	Name        string           `yaml:"name" json:"name"`
	Description string           `yaml:"description,omitempty" json:"description,omitempty"`
	Request     *HttpRequest     `yaml:"request,omitempty" json:"request,omitempty"`
	Response    *ExampleResponse `yaml:"response,omitempty" json:"response,omitempty"`
}

// ExampleResponse is the stored response half of an example.
type ExampleResponse struct {
	Status     int           `yaml:"status" json:"status"`
	StatusText string        `yaml:"statusText,omitempty" json:"statusText,omitempty"`
	Headers    []KV          `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body       *ResponseBody `yaml:"body,omitempty" json:"body,omitempty"`
}

// ExamplesAsList decodes the raw examples node into typed examples.
func (r *RequestDoc) ExamplesAsList() []HttpRequestExample {
	var out []HttpRequestExample
	if r != nil && r.Examples.Node.Kind == yaml.SequenceNode {
		_ = r.Examples.Node.Decode(&out)
	}
	return out
}

// SetExamples replaces the raw examples node with the typed list.
func (r *RequestDoc) SetExamples(ex []HttpRequestExample) error {
	if len(ex) == 0 {
		r.Examples = YamlRaw{}
		return nil
	}
	var n yaml.Node
	if err := n.Encode(ex); err != nil {
		return err
	}
	r.Examples = YamlRaw{Node: n}
	return nil
}
