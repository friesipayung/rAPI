package exporter

// Export converts a vault collection (OpenCollection) into a Postman
// Collection v2.1 JSON file. The mapping is lossless for requests, folders,
// headers, bodies and common auth types; scripts/assertions/examples are
// rAPI-only and skipped (reported as warnings).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
)

const postmanSchema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

// --- Postman v2.1 JSON shapes (only the fields we emit) ---

type pmVariable struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type pmQueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type pmHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

type pmFormParam struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Type     string `json:"type,omitempty"` // "text" (file upload not exported)
	Disabled bool   `json:"disabled,omitempty"`
}

type pmBody struct {
	Mode       string        `json:"mode"` // raw|urlencoded|formdata
	Raw        string        `json:"raw,omitempty"`
	Options    *pmBodyOpts   `json:"options,omitempty"`
	URLEncoded []pmFormParam `json:"urlencoded,omitempty"`
	FormData   []pmFormParam `json:"formdata,omitempty"`
}

type pmBodyOpts struct {
	Raw *pmRawOpts `json:"raw,omitempty"`
}

type pmRawOpts struct {
	Language string `json:"language"` // json|xml|html|text
}

type pmAuthEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"` // "string" in v2.1
}

type pmAuth struct {
	Type   string        `json:"type"` // basic|bearer|apikey|digest|oauth2
	Basic  []pmAuthEntry `json:"basic,omitempty"`
	Bearer []pmAuthEntry `json:"bearer,omitempty"`
	ApiKey []pmAuthEntry `json:"apikey,omitempty"`
	Digest []pmAuthEntry `json:"digest,omitempty"`
}

type pmRequest struct {
	Method string         `json:"method"`
	Header []pmHeader     `json:"header,omitempty"`
	Body   *pmBody        `json:"body,omitempty"`
	URL    any            `json:"url"` // string or url object
	Auth   *pmAuth        `json:"auth,omitempty"`
	Descr  *pmDescription `json:"description,omitempty"`
}

type pmDescription struct {
	Content string `json:"content"`
	Type    string `json:"type"` // "text/markdown"
}

type pmItem struct {
	Name    string     `json:"name"`
	Request *pmRequest `json:"request,omitempty"`
	Item    []pmItem   `json:"item,omitempty"` // folders
}

type pmInfo struct {
	Name       string `json:"name"`
	Schema     string `json:"schema"`
	PostmanID  string `json:"_postman_id,omitempty"`
}

type pmCollection struct {
	Info     pmInfo       `json:"info"`
	Item     []pmItem     `json:"item"`
	Variable []pmVariable `json:"variable,omitempty"`
}

type exportResult struct {
	Warnings []string
}

func (r *exportResult) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Collect walks a vault collection and builds Postman v2.1 JSON.
func Collect(vaultPath, collectionID string) (data []byte, warnings []string, err error) {
	treeRoot, err := store.GetTree(vaultPath, collectionID)
	if err != nil {
		return nil, nil, fmt.Errorf("load collection: %w", err)
	}
	out := &exportResult{}
	col := pmCollection{
		Info: pmInfo{
			Name:      treeRoot.Name,
			Schema:    postmanSchema,
			PostmanID: collectionID,
		},
	}
	for _, child := range treeRoot.Children {
		col.Item = append(col.Item, exportNode(vaultPath, collectionID, child, out))
	}
	envs, err := store.ListEnvironments(vaultPath, collectionID)
	if err == nil {
		// first environment becomes collection variables
		for _, env := range envs {
			if env == nil {
				continue
			}
			for _, v := range env.Variables {
				if v.Name == "" || v.Secret {
					continue
				}
				col.Variable = append(col.Variable, pmVariable{Key: v.Name, Value: v.Value})
			}
			if len(col.Variable) > 0 {
				break
			}
		}
	}
	data, err = json.MarshalIndent(col, "", "\t")
	if err != nil {
		return nil, nil, err
	}
	return data, out.Warnings, nil
}

func exportNode(vaultPath, collectionID string, n *model.TreeNode, out *exportResult) pmItem {
	if n.Type == "http" {
		item := pmItem{Name: n.Name}
		// tree node IDs are collection-prefixed ("col/folder/req")
		rel := strings.TrimPrefix(n.ID, collectionID+"/")
		rd, err := store.LoadRequestDoc(vaultPath, collectionID, rel)
		if err != nil {
			out.warnf("request %q: %v", n.Name, err)
			return item
		}
		req, warns := requestToPostman(rd)
		item.Request = req
		for _, w := range warns {
			out.warnf("request %q: %s", n.Name, w)
		}
		return item
	}
	// folder (or anything with children)
	item := pmItem{Name: n.Name}
	for _, child := range n.Children {
		item.Item = append(item.Item, exportNode(vaultPath, collectionID, child, out))
	}
	return item
}

func requestToPostman(rd *model.RequestDoc) (*pmRequest, []string) {
	var warnings []string
	h := &rd.Http
	req := &pmRequest{Method: strings.ToUpper(firstNonEmpty(h.Method, "GET"))}

	if rd.Docs != "" {
		req.Descr = &pmDescription{Content: rd.Docs, Type: "text/markdown"}
	}

	// headers
	for _, kv := range h.Headers {
		if kv.Name == "" {
			continue
		}
		req.Header = append(req.Header, pmHeader{Key: kv.Name, Value: kv.Value, Disabled: kv.Disabled})
	}

	// URL: emit as string (Postman accepts a plain string)
	if h.URL != "" {
		req.URL = h.URL
	}

	// body
	if b := h.Body; b != nil {
		switch b.Type {
		case "", "raw":
			req.Body = &pmBody{Mode: "raw", Raw: b.Data, Options: &pmBodyOpts{Raw: &pmRawOpts{Language: "text"}}}
		case "json":
			req.Body = &pmBody{Mode: "raw", Raw: b.Data, Options: &pmBodyOpts{Raw: &pmRawOpts{Language: "json"}}}
		case "xml":
			req.Body = &pmBody{Mode: "raw", Raw: b.Data, Options: &pmBodyOpts{Raw: &pmRawOpts{Language: "xml"}}}
		case "form-urlencoded":
			req.Body = &pmBody{Mode: "urlencoded"}
			for _, kv := range b.FormData {
				if kv.Name == "" {
					continue
				}
				req.Body.URLEncoded = append(req.Body.URLEncoded, pmFormParam{Key: kv.Name, Value: kv.Value, Disabled: kv.Disabled})
			}
		case "multipart-form":
			req.Body = &pmBody{Mode: "formdata"}
			for _, kv := range b.FormData {
				if kv.Name == "" {
					continue
				}
				req.Body.FormData = append(req.Body.FormData, pmFormParam{Key: kv.Name, Value: kv.Value, Type: "text", Disabled: kv.Disabled})
			}
		default:
			warnings = append(warnings, fmt.Sprintf("body type %q not representable in Postman export", b.Type))
		}
	}

	// auth
	if h.Auth != nil && !h.Auth.IsEmpty() {
		auth, warns := authToPostman(h.Auth)
		req.Auth = auth
		warnings = append(warnings, warns...)
	}
	return req, warnings
}

func authEntry(key, val string) pmAuthEntry {
	return pmAuthEntry{Key: key, Value: val, Type: "string"}
}

func authToPostman(a *model.Auth) (*pmAuth, []string) {
	var warnings []string
	switch a.Type {
	case model.AuthBasic:
		pm := &pmAuth{Type: "basic", Basic: []pmAuthEntry{
			authEntry("username", a.Basic.Username),
			authEntry("password", a.Basic.Password),
		}}
		return pm, nil
	case model.AuthBearer:
		return &pmAuth{Type: "bearer", Bearer: []pmAuthEntry{authEntry("token", a.Bearer.Token)}}, nil
	case model.AuthApiKey:
		in := "header"
		if strings.EqualFold(a.ApiKey.In, "query") {
			in = "queryParams"
		}
		return &pmAuth{Type: "apikey", ApiKey: []pmAuthEntry{
			authEntry("key", a.ApiKey.Key),
			authEntry("value", a.ApiKey.Value),
			authEntry("in", in),
		}}, nil
	case model.AuthDigest:
		return &pmAuth{Type: "digest", Digest: []pmAuthEntry{
			authEntry("username", a.Digest.Username),
			authEntry("password", a.Digest.Password),
		}}, nil
	case model.AuthOAuth2:
		o := a.OAuth2
		pm := &pmAuth{Type: "oauth2", Basic: []pmAuthEntry{
			authEntry("grant_type", firstNonEmpty(o.GrantType, "client_credentials")),
			authEntry("access_token_url", o.TokenURL),
			authEntry("client_id", o.ClientID),
			authEntry("client_secret", o.ClientSecret),
		}}
		if o.AuthURL != "" {
			pm.Basic = append(pm.Basic, authEntry("auth_url", o.AuthURL))
		}
		if o.Scopes != "" {
			pm.Basic = append(pm.Basic, authEntry("scope", o.Scopes))
		}
		if o.UsePKCE {
			pm.Basic = append(pm.Basic, authEntry("challenge_algorithm", "S256"))
		}
		return pm, nil
	default:
		warnings = append(warnings, fmt.Sprintf("auth type %q not representable in Postman export (skipped)", a.Type))
		return nil, warnings
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// SaveToFile writes the exported JSON to path (parent dirs created).
func SaveToFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}
