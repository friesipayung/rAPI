package importer

// Insomnia v4 export JSON → OpenCollection.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

type insomniaExport struct {
	Resources []insomniaResource `json:"resources"`
}

type insomniaResource struct {
	ID       string          `json:"_id"`
	Type     string          `json:"_type"` // workspace|folder|request|environment
	Name     string          `json:"name"`
	ParentID string          `json:"parentId"`
	URL      string          `json:"url"`
	Method   string          `json:"method"`
	Headers  []insomniaKV    `json:"headers"`
	Params   []insomniaKV    `json:"parameters"`
	Body     *insomniaBody   `json:"body,omitempty"`
	Auth     *insomniaAuth   `json:"authentication,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"` // environment vars
	MetaJSON string          `json:"metaJSON,omitempty"`
}

type insomniaKV struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

type insomniaBody struct {
	MimeType string        `json:"mimeType,omitempty"` // application/json, text/plain, form-urlencoded, multipart/form-data
	Text     string        `json:"text,omitempty"`
	Params   []insomniaKV  `json:"params,omitempty"`
}

type insomniaAuth struct {
	Type       string `json:"type,omitempty"` // basic|bearer|apikey|digest|oauth2|...
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	Token      string `json:"token,omitempty"`
	Key        string `json:"key,omitempty"`
	Value      string `json:"value,omitempty"`
	Placement  string `json:"placement,omitempty"` // query|header (apikey)
	Disabled   bool   `json:"disabled,omitempty"`
}

func parseInsomnia(path string) (*tree, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ex insomniaExport
	if err := parseJSON(b, &ex); err != nil {
		return nil, fmt.Errorf("invalid Insomnia JSON: %w", err)
	}

	byID := map[string]*insomniaResource{}
	var workspace *insomniaResource
	for i := range ex.Resources {
		r := &ex.Resources[i]
		byID[r.ID] = r
		if r.Type == "workspace" && workspace == nil {
			workspace = r
		}
	}
	if workspace == nil {
		return nil, fmt.Errorf("no workspace in export (is this an Insomnia v4 export?)")
	}

	root := &tree{name: workspace.Name}
	children := map[string]*tree{} // folderID → node

	// pass 1: environments (workspace-level vars)
	for i := range ex.Resources {
		r := &ex.Resources[i]
		if r.Type == "environment" && r.ParentID == workspace.ID {
			root.vars = append(root.vars, insomniaEnvVars(r)...)
		}
	}
	// pass 2: folders (nested by parentId chain)
	var addFolders func(parentID string, parent *tree)
	addFolders = func(parentID string, parent *tree) {
		for _, r := range ex.Resources {
			if r.Type == "folder" && r.ParentID == parentID {
				n := &tree{name: r.Name}
				children[r.ID] = n
				parent.children = append(parent.children, n)
				addFolders(r.ID, n)
			}
		}
	}
	addFolders(workspace.ID, root)

	// pass 3: requests
	for i := range ex.Resources {
		r := &ex.Resources[i]
		if r.Type != "request" {
			continue
		}
		rd := insomniaToRequest(r)
		var parent *tree
		if strings.HasPrefix(r.ParentID, "fld_") {
			parent = children[r.ParentID]
		}
		if parent == nil {
			parent = root
		}
		parent.requests = append(parent.requests, rd)
	}
	return root, nil
}

func insomniaEnvVars(r *insomniaResource) []model.Variable {
	var out []model.Variable
	if len(r.Data) == 0 {
		return out
	}
	var m map[string]any
	if parseJSON(r.Data, &m) != nil {
		return out
	}
	for k, v := range m {
		out = append(out, model.Variable{Name: k, Value: fmt.Sprintf("%v", v)})
	}
	return out
}

func insomniaToRequest(r *insomniaResource) *model.RequestDoc {
	rd := &model.RequestDoc{}
	rd.Info.Name = r.Name
	rd.Info.Type = "http"
	h := &rd.Http
	h.Method = strings.ToUpper(firstNonEmptyStr(r.Method, "GET"))

	// Insomnia puts the query string inside the URL already
	u, err := url.Parse(r.URL)
	if err == nil && u.RawQuery != "" {
		for k, vs := range u.Query() {
			for _, v := range vs {
				h.Params = append(h.Params, model.KV{Name: k, Value: v})
			}
		}
		// keep params as {{vars}}-friendly URL without query
		u.RawQuery = ""
		h.URL = u.String()
	} else {
		h.URL = r.URL
	}

	for _, hd := range r.Headers {
		h.Headers = append(h.Headers, model.KV{Name: hd.Name, Value: hd.Value, Disabled: hd.Disabled})
	}
	if r.Body != nil {
		switch r.Body.MimeType {
		case "application/json":
			h.Body = &model.Body{Type: model.BodyJSON, Data: r.Body.Text}
		case "application/xml", "text/xml":
			h.Body = &model.Body{Type: model.BodyXML, Data: r.Body.Text}
		case "text/html":
			h.Body = &model.Body{Type: model.BodyHTML, Data: r.Body.Text}
		case "form-urlencoded":
			h.Body = &model.Body{Type: model.BodyForm}
			for _, p := range r.Body.Params {
				h.Body.FormData = append(h.Body.FormData, model.KV{Name: p.Name, Value: p.Value, Disabled: p.Disabled})
			}
		case "multipart/form-data":
			h.Body = &model.Body{Type: model.BodyMultipart}
			for _, p := range r.Body.Params {
				h.Body.FormData = append(h.Body.FormData, model.KV{Name: p.Name, Value: firstNonEmptyStr(p.Value, "file:"+p.Name), Disabled: p.Disabled})
			}
		case "", "text/plain":
			h.Body = &model.Body{Type: model.BodyText, Data: r.Body.Text}
		default:
			h.Body = &model.Body{Type: model.BodyText, Data: r.Body.Text}
		}
	}
	h.Auth = insomniaToAuth(r.Auth)
	return rd
}

func insomniaToAuth(a *insomniaAuth) *model.Auth {
	if a == nil || a.Disabled {
		return nil
	}
	switch a.Type {
	case "basic":
		return &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{Username: a.Username, Password: a.Password}}
	case "bearer", "bearer-1":
		return &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: a.Token}}
	case "apikey", "apikey-1":
		in := "header"
		if a.Placement == "query" {
			in = "query"
		}
		return &model.Auth{Type: model.AuthApiKey, ApiKey: &model.ApiKeyAuth{Key: a.Key, Value: a.Value, In: in}}
	case "digest", "digest-1":
		return &model.Auth{Type: model.AuthDigest, Digest: &model.DigestAuth{Username: a.Username, Password: a.Password}}
	}
	return nil // oauth2/ntlamazon etc. arrive in phase 4
}
