package importer

// Bruno (.bru) → OpenCollection. Bruno folders are plain directories holding
// .bru text files; folder.bru files hold folder metadata/vars.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

func parseBruno(path string) (*tree, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	root := &tree{name: "Imported Bruno"}
	if st.IsDir() {
		walkBrunoDir(path, root)
	} else {
		rd, err := parseBruFile(path)
		if err != nil {
			return nil, err
		}
		root.requests = append(root.requests, rd)
	}
	if len(root.requests) == 0 && len(root.children) == 0 {
		return nil, fmt.Errorf("no .bru files found in %q", filepath.Base(path))
	}
	return root, nil
}

func walkBrunoDir(dir string, parent *tree) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			ch := &tree{name: e.Name()}
			walkBrunoDir(full, ch)
			if len(ch.requests) > 0 || len(ch.children) > 0 {
				parent.children = append(parent.children, ch)
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), ".bru") {
			continue
		}
		if e.Name() == "folder.bru" {
			if vars := bruFolderVars(full); len(vars) > 0 {
				parent.vars = append(parent.vars, vars...)
			}
			continue
		}
		rd, err := parseBruFile(full)
		if err != nil {
			continue // skip broken files silently
		}
		parent.requests = append(parent.requests, rd)
	}
}

// bruNode is one top-level block of a .bru file.
type bruNode struct {
	kind  string
	lines []string
}

// parseBruBlocks splits a .bru file into its top-level blocks.
func parseBruBlocks(path string) ([]bruNode, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var nodes []bruNode
	var stack []*bruNode // stack[0] = current top-level block; deeper = nested
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, "{") {
			if len(stack) == 0 {
				stack = append(stack, &bruNode{kind: strings.TrimSpace(strings.TrimSuffix(trimmed, "{"))})
			} else {
				// nested block (auth > bearer, body:form > urlencoded): flatten
				// marker + contents into the enclosing top-level block
				stack[0].lines = append(stack[0].lines, trimmed)
				stack = append(stack, &bruNode{})
			}
			continue
		}
		if trimmed == "}" {
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				nodes = append(nodes, *top)
			}
			continue
		}
		if len(stack) > 0 {
			stack[0].lines = append(stack[0].lines, trimmed)
		}
	}
	return nodes, sc.Err()
}

// bruKV parses "key: value" lines (bruno strips quotes).
func bruKV(line string) (string, string, bool) {
	i := strings.Index(line, ":")
	if i <= 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:i])
	v := strings.TrimSpace(line[i+1:])
	v = strings.Trim(v, `"'`)
	return k, v, true
}

func bruMultiLineKV(lines []string, i int) (string, string, int) {
	k, _, ok := bruKV(lines[i])
	if !ok {
		return "", "", i
	}
	// multi-line: value opens with > on subsequent lines
	var sb strings.Builder
	j := i + 1
	for j < len(lines) && strings.HasPrefix(lines[j], ">") {
		sb.WriteString(strings.TrimPrefix(lines[j], ">"))
		sb.WriteString("\n")
		j++
	}
	if sb.Len() > 0 {
		return k, strings.TrimRight(sb.String(), "\n"), j - 1
	}
	// single-line value was already consumed by bruKV; reparse
	if _, v, ok := bruKV(lines[i]); ok {
		return k, v, i
	}
	return k, "", i
}

func parseBruFile(path string) (*model.RequestDoc, error) {
	nodes, err := parseBruBlocks(path)
	if err != nil {
		return nil, err
	}
	rd := &model.RequestDoc{}
	rd.Info.Type = "http"
	var docs strings.Builder
	var vars []model.Variable

	for _, n := range nodes {
		switch n.kind {
		case "meta":
			for _, l := range n.lines {
				if k, v, ok := bruKV(l); ok {
					switch k {
					case "name":
						rd.Info.Name = v
					case "seq":
						fmt.Sscanf(v, "%d", &rd.Info.Seq)
					case "description":
						rd.Info.Description = v
					}
				}
			}
		case "get", "post", "put", "patch", "delete", "head", "options", "connect", "trace":
			rd.Http.Method = strings.ToUpper(n.kind)
			for _, l := range n.lines {
				if k, v, ok := bruKV(l); ok && k == "url" {
					rd.Http.URL = v
				}
			}
		case "query":
			for _, l := range n.lines {
				if k, v, ok := bruKV(l); ok {
					disabled := strings.HasPrefix(l, "~")
					k = strings.TrimPrefix(k, "~")
					rd.Http.Params = append(rd.Http.Params, model.KV{Name: k, Value: v, Disabled: disabled})
				}
			}
		case "headers":
			for _, l := range n.lines {
				if k, v, ok := bruKV(l); ok {
					disabled := strings.HasPrefix(l, "~")
					k = strings.TrimPrefix(k, "~")
					rd.Http.Headers = append(rd.Http.Headers, model.KV{Name: k, Value: v, Disabled: disabled})
				}
			}
		case "auth":
			rd.Http.Auth = bruAuth(n.lines)
		case "body", "body:form", "body:json", "body:text", "body:xml", "body:html", "body:graphql", "body:graphql-vars":
			bruBody(rd, n.kind, n.lines)
		case "docs":
			for _, l := range n.lines {
				docs.WriteString(strings.TrimPrefix(l, "> "))
				docs.WriteString("\n")
			}
		case "vars", "vars:secret":
			for _, l := range n.lines {
				secret := n.kind == "vars:secret"
				if k, v, ok := bruKV(l); ok {
					vars = append(vars, model.Variable{Name: k, Value: v, Secret: secret})
				}
			}
		}
	}
	if rd.Info.Name == "" {
		rd.Info.Name = strings.TrimSuffix(filepath.Base(path), ".bru")
	}
	if docs.Len() > 0 {
		rd.Docs = strings.TrimSpace(docs.String())
	}
	_ = vars // request-level vars have no OpenCollection slot yet; folder vars handled above
	return rd, nil
}

func bruAuth(lines []string) *model.Auth {
	// flattened nested lines, e.g. ["bearer {", "token: xxx", "}"]
	var auth *model.Auth
	var kind string
	for _, l := range lines {
		if strings.HasSuffix(l, "{") {
			kind = strings.TrimSpace(strings.TrimSuffix(l, "{"))
			switch kind {
			case "basic", "digest", "bearer", "apikey":
				auth = &model.Auth{Type: kind}
			default:
				kind = ""
			}
			continue
		}
		if l == "}" {
			kind = ""
			continue
		}
		if auth == nil || kind == "" {
			continue
		}
		k, v, ok := bruKV(l)
		if !ok {
			continue
		}
		switch auth.Type {
		case "basic", "digest":
			if auth.Basic == nil {
				auth.Basic = &model.BasicAuth{}
			}
			if k == "username" {
				auth.Basic.Username = v
			} else if k == "password" {
				auth.Basic.Password = v
			}
			if auth.Type == "digest" {
				auth.Digest = &model.DigestAuth{Username: auth.Basic.Username, Password: auth.Basic.Password}
			}
		case "bearer":
			if auth.Bearer == nil {
				auth.Bearer = &model.BearerAuth{}
			}
			if k == "token" {
				auth.Bearer.Token = v
			}
		case "apikey":
			if auth.ApiKey == nil {
				auth.ApiKey = &model.ApiKeyAuth{}
			}
			switch k {
			case "key":
				auth.ApiKey.Key = v
			case "value":
				auth.ApiKey.Value = v
			case "in":
				auth.ApiKey.In = v
			}
		}
	}
	return auth
}

func bruBody(rd *model.RequestDoc, kind string, lines []string) {
	b := &model.Body{Type: "text"}
	switch kind {
	case "body:json":
		b.Type = model.BodyJSON
	case "body:xml":
		b.Type = model.BodyXML
	case "body:html":
		b.Type = model.BodyHTML
	case "body:graphql":
		b.Type = "text" // graphql arrives with phase 4 scripting
	}
	var data strings.Builder
	formMode := false
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		key := strings.TrimSpace(strings.TrimSuffix(l, "{"))
		if l == "formdata" || l == "multipart" || key == "urlencoded" || strings.HasSuffix(l, "{") {
			if key == "urlencoded" {
				b.Type = model.BodyForm
				formMode = true
			} else if l == "formdata" || l == "multipart" || key == "formdata" || key == "multipart" {
				b.Type = model.BodyMultipart
				formMode = true
			}
			continue
		}
		if formMode {
			if k, v, ok := bruKV(l); ok {
				b.FormData = append(b.FormData, model.KV{Name: k, Value: v})
			}
			continue
		}
		data.WriteString(strings.TrimPrefix(l, "> "))
		data.WriteString("\n")
	}
	if !formMode {
		b.Data = strings.TrimSpace(data.String())
	}
	rd.Http.Body = b
}

func bruFolderVars(path string) []model.Variable {
	nodes, err := parseBruBlocks(path)
	if err != nil {
		return nil
	}
	var out []model.Variable
	for _, n := range nodes {
		if n.kind != "vars" && n.kind != "vars:secret" {
			continue
		}
		secret := n.kind == "vars:secret"
		for _, l := range n.lines {
			if k, v, ok := bruKV(l); ok {
				out = append(out, model.Variable{Name: k, Value: v, Secret: secret})
			}
		}
	}
	return out
}
