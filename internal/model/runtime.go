package model

// Phase 4 runtime: pre/post scripts, assertions, proxy and OAuth2 auth.

const (
	ScriptPre  = "pre"
	ScriptPost = "post"
)

// Script is a JavaScript snippet executed around the request (goja).
type Script struct {
	Type    string `yaml:"type" json:"type"` // pre|post
	Content string `yaml:"content" json:"content"`
}

// Assertion is a declarative check evaluated against the response.
type Assertion struct {
	Expression string `yaml:"expression" json:"expression"` // e.g. res.status, res.body.id
	Operator   string `yaml:"operator" json:"operator"`     // equal|notEqual|contains|matches|greaterThan|lessThan|exists
	Value      string `yaml:"value,omitempty" json:"value,omitempty"`
	Disabled   bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// Runtime holds scripts and assertions on a request file.
type Runtime struct {
	Scripts    []Script    `yaml:"scripts,omitempty" json:"scripts,omitempty"`
	Assertions []Assertion `yaml:"assertions,omitempty" json:"assertions,omitempty"`
}

// AssertionResult is the outcome of one assertion run.
type AssertionResult struct {
	Expression string `json:"expression"`
	Operator   string `json:"operator"`
	Expected   string `json:"expected,omitempty"`
	Actual     string `json:"actual,omitempty"`
	Pass       bool   `json:"pass"`
	Error      string `json:"error,omitempty"`
}

// ProxyConfig routes requests through an HTTP(S) proxy.
type ProxyConfig struct {
	URL      string `yaml:"url" json:"url"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

const (
	AuthOAuth2 = "oauth2"
)

// OAuth2Auth supports the client_credentials and password grants.
type OAuth2Auth struct {
	GrantType    string `yaml:"grantType" json:"grantType"` // client_credentials|password
	TokenURL     string `yaml:"tokenUrl" json:"tokenUrl"`
	ClientID     string `yaml:"clientId,omitempty" json:"clientId,omitempty"`
	ClientSecret string `yaml:"clientSecret,omitempty" json:"clientSecret,omitempty"`
	Username     string `yaml:"username,omitempty" json:"username,omitempty"`
	Password     string `yaml:"password,omitempty" json:"password,omitempty"`
	Scopes       string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}
