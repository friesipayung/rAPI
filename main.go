package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/sync"
	"github.com/friesipayung/rapi/internal/vault"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// SyncEmitter forwards sync status changes to the frontend via the
// `sync-status` event (set once the app exists).
var SyncEmitter func(model.SyncStatus)

func init() {
	application.RegisterEvent[model.SyncStatus]("sync-status")
}

// main wires the git engine, vault manager and syncer, binds the services and
// runs the app.
func main() {
	g := git.Cli{}
	mgr := vault.New()
	syncer := sync.New(g)

	app := application.New(application.Options{
		Name:        "rAPI",
		Description: "API client with git as the vault",
		Services: []application.Service{
			application.NewService(NewVaultService(mgr, g, syncer)),
			application.NewService(NewSyncService(mgr, syncer)),
			application.NewService(NewTreeService(mgr)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Fan sync status out to every window.
	SyncEmitter = func(st model.SyncStatus) {
		app.Event.Emit("sync-status", st)
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "rAPI",
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(247, 248, 250),
		URL:              "/",
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
