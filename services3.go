package main

// Phase 3 services: import third-party collections (Postman v2.1, Bruno,
// Insomnia v4) into the vault, and validate collection layout.

import (
	"context"
	"fmt"

	"github.com/friesipayung/rapi/internal/codegen"
	"github.com/friesipayung/rapi/internal/importer"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
	"github.com/friesipayung/rapi/internal/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ImportService struct {
	mgr *vault.Manager
}

func NewImportService(mgr *vault.Manager) *ImportService {
	return &ImportService{mgr: mgr}
}

// PickAndImport opens a native file/folder dialog, detects the format and
// writes the converted collection into the vault.
func (s *ImportService) PickAndImport(ctx context.Context) (*importer.Result, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Import collection — Postman v2.1 JSON, Bruno folder, Insomnia v4 JSON").
		CanChooseFiles(true).
		CanChooseDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	return importer.ImportFromPath(dir, path, "auto", "")
}

// Validate checks the layout of one imported collection (read-only).
func (s *ImportService) Validate(ctx context.Context, collectionID string) (*importer.Report, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	return importer.ValidateCollection(dir, collectionID), nil
}

// ---------- examples (phase 3) ----------

// SaveAsExample stores the last run (request snapshot + response) as a named
// example on the request file, natively under examples[].
func (s *RunService) SaveAsExample(ctx context.Context, requestID string, result model.RunResult, name string) (*model.RunResult, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	collectionID, rel, err := splitID(requestID)
	if err != nil {
		return nil, err
	}
	doc, err := store.LoadRequestDoc(dir, collectionID, rel)
	if err != nil {
		return nil, err
	}
	ex := model.HttpRequestExample{Name: name}
	if result.Request != nil {
		ex.Request = &model.HttpRequest{
			Method:  result.Request.Method,
			URL:     result.Request.URL,
			Params:  result.Request.Params,
			Headers: result.Request.Headers,
			Body:    result.Request.Body,
		}
	}
	if result.Status != 0 || result.Body != nil {
		ex.Response = &model.ExampleResponse{
			Status:     result.Status,
			StatusText: result.StatusText,
			Headers:    result.Headers,
			Body:       result.Body,
		}
	}
	list := doc.ExamplesAsList()
	list = append(list, ex)
	if err := doc.SetExamples(list); err != nil {
		return nil, err
	}
	if err := store.SaveRequestDoc(dir, collectionID, rel, doc); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListExamples returns the examples stored on a request.
func (s *RequestService) ListExamples(ctx context.Context, requestID string) ([]*model.HttpRequestExample, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return nil, vault.ErrNoVault
	}
	collectionID, rel, err := splitID(requestID)
	if err != nil {
		return nil, err
	}
	doc, err := store.LoadRequestDoc(dir, collectionID, rel)
	if err != nil {
		return nil, err
	}
	out := []*model.HttpRequestExample{}
	list := doc.ExamplesAsList()
	for i := range list {
		out = append(out, &list[i])
	}
	return out, nil
}

// DeleteExample removes one example by index.
func (s *RequestService) DeleteExample(ctx context.Context, requestID string, index int) error {
	dir := s.mgr.Dir()
	if dir == "" {
		return vault.ErrNoVault
	}
	collectionID, rel, err := splitID(requestID)
	if err != nil {
		return err
	}
	doc, err := store.LoadRequestDoc(dir, collectionID, rel)
	if err != nil {
		return err
	}
	list := doc.ExamplesAsList()
	if index < 0 || index >= len(list) {
		return fmt.Errorf("example index %d out of range", index)
	}
	list = append(list[:index], list[index+1:]...)
	if err := doc.SetExamples(list); err != nil {
		return err
	}
	return store.SaveRequestDoc(dir, collectionID, rel, doc)
}

// ---------- code generation ----------

type CodegenService struct {
	mgr *vault.Manager
}

func NewCodegenService(mgr *vault.Manager) *CodegenService {
	return &CodegenService{mgr: mgr}
}

// Targets lists the supported languages/clients.
func (s *CodegenService) Targets(ctx context.Context) []codegen.Target {
	return codegen.Targets()
}

// Generate produces client code for a request in the chosen target.
func (s *CodegenService) Generate(ctx context.Context, requestID, target string) (string, error) {
	dir := s.mgr.Dir()
	if dir == "" {
		return "", vault.ErrNoVault
	}
	collectionID, rel, err := splitID(requestID)
	if err != nil {
		return "", err
	}
	doc, err := store.LoadRequestDoc(dir, collectionID, rel)
	if err != nil {
		return "", err
	}
	return codegen.Generate(target, &doc.Http, doc.Settings)
}
