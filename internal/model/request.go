package model

import "gopkg.in/yaml.v3"

// ---------- full request / environment models (phase 2) ----------
// Loose but typed: unknown fields inside `examples` / `runtime` are preserved
// verbatim via yaml.Node so phase 3 can add them without a migration.

// KV is a named value used for params, headers, form fields, variables.
type KV struct {
	Name        string `yaml:"name" json:"name"`
	Value       string `yaml:"value,omitempty" json:"value"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"` // query|path|header (params)
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Disabled    bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Secret      bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// Variable is a scoped variable (collection/folder/environment).
type Variable struct {
	Name        string `yaml:"name" json:"name"`
	Value       string `yaml:"value,omitempty" json:"value"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Secret      bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
	Disabled    bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// ---------- auth ----------

// Auth types supported in phase 1–2.
const (
	AuthInherit = "inherit"
	AuthBasic   = "basic"
	AuthBearer  = "bearer"
	AuthApiKey  = "apikey"
	AuthDigest  = "digest"
)

type BasicAuth struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password,omitempty" json:"password"`
}

type BearerAuth struct {
	Token string `yaml:"token" json:"token"`
}

type ApiKeyAuth struct {
	Key   string `yaml:"key" json:"key"`
	Value string `yaml:"value" json:"value"`
	In    string `yaml:"in,omitempty" json:"in"` // header|query
}

type DigestAuth struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password,omitempty" json:"password"`
}

type Auth struct {
	Type   string      `yaml:"type,omitempty" json:"type"`
	Basic  *BasicAuth  `yaml:"basic,omitempty" json:"basic,omitempty"`
	Bearer *BearerAuth `yaml:"bearer,omitempty" json:"bearer,omitempty"`
	ApiKey *ApiKeyAuth `yaml:"apikey,omitempty" json:"apikey,omitempty"`
	Digest *DigestAuth `yaml:"digest,omitempty" json:"digest,omitempty"`
	OAuth2 *OAuth2Auth `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`
	AWSV4  *AWSV4Auth  `yaml:"awsv4,omitempty" json:"awsv4,omitempty"`
	WSSE   *WSSEAuth   `yaml:"wsse,omitempty" json:"wsse,omitempty"`
}

// IsEmpty reports whether the auth carries no credentials.
func (a *Auth) IsEmpty() bool {
	if a == nil {
		return true
	}
	switch a.Type {
	case AuthBasic:
		return a.Basic == nil || a.Basic.Username == ""
	case AuthBearer:
		return a.Bearer == nil || a.Bearer.Token == ""
	case AuthApiKey:
		return a.ApiKey == nil || a.ApiKey.Key == ""
	case AuthDigest:
		return a.Digest == nil || a.Digest.Username == ""
	case AuthOAuth2:
		return a.OAuth2 == nil || a.OAuth2.TokenURL == ""
	case AuthAWSV4:
		return a.AWSV4 == nil || a.AWSV4.AccessKey == ""
	case AuthNTLM:
		return false
	case AuthWSSE:
		return a.WSSE == nil || a.WSSE.Username == ""
	}
	return true
}

// ---------- body ----------

// Body types.
const (
	BodyJSON    = "json"
	BodyText    = "text"
	BodyXML     = "xml"
	BodyHTML    = "html"
	BodyForm    = "form-urlencoded"
	BodyMultipart = "multipart-form"
)

type Body struct {
	Type     string `yaml:"type,omitempty" json:"type"`
	Data     string `yaml:"data,omitempty" json:"data"`
	FormData []KV   `yaml:"form,omitempty" json:"form,omitempty"` // form-urlencoded & multipart fields
}

// ---------- settings ----------

type Settings struct {
	EncodeUrl       *bool            `yaml:"encodeUrl,omitempty" json:"encodeUrl,omitempty"`
	Timeout         int              `yaml:"timeout,omitempty" json:"timeout,omitempty"` // seconds; 0 = default 30s
	FollowRedirects *bool            `yaml:"followRedirects,omitempty" json:"followRedirects,omitempty"`
	MaxRedirects    int              `yaml:"maxRedirects,omitempty" json:"maxRedirects,omitempty"`
	Proxy           *ProxyConfig     `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	ClientCert      *ClientCertConfig `yaml:"clientCert,omitempty" json:"clientCert,omitempty"`
}

// ---------- http request ----------

type HttpRequest struct {
	Method  string `yaml:"method" json:"method"`
	URL     string `yaml:"url" json:"url"`
	Params  []KV   `yaml:"params,omitempty" json:"params,omitempty"`
	Headers []KV   `yaml:"headers,omitempty" json:"headers,omitempty"`
	Auth    *Auth  `yaml:"auth,omitempty" json:"auth,omitempty"`
	Body    *Body  `yaml:"body,omitempty" json:"body,omitempty"`
}

type RequestDoc struct {
	Info     ItemInfo    `yaml:"info" json:"info"`
	Http     HttpRequest `yaml:"http" json:"http"`
	Examples YamlRaw     `yaml:"examples,omitempty" json:"examples,omitempty"`
	Runtime  YamlRaw     `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Settings *Settings   `yaml:"settings,omitempty" json:"settings,omitempty"`
	Docs     string      `yaml:"docs,omitempty" json:"docs,omitempty"`
}

// YamlRaw preserves an arbitrary YAML value across load/save roundtrips.
type YamlRaw struct{ Node yaml.Node }

func (y YamlRaw) MarshalYAML() (interface{}, error) {
	if y.Node.Kind == 0 {
		return nil, nil
	}
	return &y.Node, nil
}

func (y *YamlRaw) UnmarshalYAML(node *yaml.Node) error { y.Node = *node; return nil }

// ---------- collection / folder ----------

type RequestDefaults struct {
	Headers   []KV       `yaml:"headers,omitempty" json:"headers,omitempty"`
	Variables []Variable `yaml:"variables,omitempty" json:"variables,omitempty"`
	Auth      *Auth      `yaml:"auth,omitempty" json:"auth,omitempty"`
	Settings  *Settings  `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// CollectionDoc is the full shape of opencollection.yml.
type CollectionDoc struct {
	Opencollection string `yaml:"opencollection" json:"opencollection"`
	Info           struct {
		Name    string `yaml:"name" json:"name"`
		Summary string `yaml:"summary,omitempty" json:"summary"`
		Version string `yaml:"version,omitempty" json:"version"`
	} `yaml:"info" json:"info"`
	Config struct {
		Environments []string        `yaml:"environments,omitempty" json:"environments"`
		Request      *RequestDefaults `yaml:"request,omitempty" json:"request,omitempty"`
	} `yaml:"config" json:"config"`
	Docs string `yaml:"docs,omitempty" json:"docs,omitempty"`
}

// FolderDoc is the full shape of folder.yml.
type FolderDoc struct {
	Info    ItemInfo         `yaml:"info" json:"info"`
	Docs    string           `yaml:"docs,omitempty" json:"docs,omitempty"`
	Request *RequestDefaults `yaml:"request,omitempty" json:"request,omitempty"`
}

// ---------- environment ----------

// EnvironmentDoc is the full shape of environments/<name>.yml.
type EnvironmentDoc struct {
	Name        string     `yaml:"name" json:"name"`
	Color       string     `yaml:"color,omitempty" json:"color,omitempty"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Variables   []Variable `yaml:"variables,omitempty" json:"variables"`
	Extends     string     `yaml:"extends,omitempty" json:"extends,omitempty"`
}

// ---------- ui state ----------

type UIState struct {
	ActiveEnvs map[string]string `yaml:"activeEnvs,omitempty" json:"activeEnvs,omitempty"`
	Theme      string            `yaml:"theme,omitempty" json:"theme,omitempty"`
}

// ---------- run result ----------

type ResponseBody struct {
	Type string `yaml:"type" json:"type"` // json|text|xml|html|binary
	Data string `yaml:"data" json:"data"`
}

type RequestSnapshot struct {
	Method  string `yaml:"method" json:"method"`
	URL     string `yaml:"url" json:"url"`
	Params  []KV   `yaml:"params,omitempty" json:"params,omitempty"`
	Headers []KV   `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    *Body  `yaml:"body,omitempty" json:"body,omitempty"`
}

type RunResult struct {
	Status         int               `json:"status"`
	StatusText     string            `json:"statusText"`
	TimeMs         int64             `json:"timeMs"`
	SizeBytes      int               `json:"sizeBytes"`
	Headers        []KV              `json:"headers"`
	Body           *ResponseBody     `json:"body"`
	Request        *RequestSnapshot  `json:"request"`
	EnvUsed        string            `json:"envUsed"`
	Error          string            `json:"error,omitempty"`
	Assertions     []AssertionResult `json:"assertions,omitempty"`
	ScriptError    string            `json:"scriptError,omitempty"`
	ConsoleLogs    []string          `json:"consoleLogs,omitempty"`
}
