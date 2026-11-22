package main

// DialogService (P0 task 7.3): native file picker for multipart form rows.
// Only the chosen PATH crosses the service boundary — the file content is
// never read here; the engine reads it at send time.

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DialogService exposes native file dialogs to the frontend.
type DialogService struct{}

func NewDialogService() *DialogService { return &DialogService{} }

// PickFile opens a native open-file dialog and returns the chosen path.
// Returns "" when the user cancels the dialog.
func (s *DialogService) PickFile(ctx context.Context) (string, error) {
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Choose file for multipart form field").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}
