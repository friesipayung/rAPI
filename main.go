package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/friesipayung/rapi/internal/git"
	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/sync"
	"github.com/friesipayung/rapi/internal/vault"
	"github.com/friesipayung/rapi/internal/version"
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
	// CLI mode: `rapi run ...` must bypass all Wails/application
	// initialization (P0 task 3.3).
	if len(os.Args) > 1 && os.Args[1] == "run" {
		os.Exit(runCLI(os.Args[2:]))
	}
	log.Printf("rAPI %s", version.Version)
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
			application.NewService(NewRequestService(mgr)),
			application.NewService(NewEnvService(mgr)),
			application.NewService(NewRunService(mgr)),
			application.NewService(NewRunnerService(mgr)),
			application.NewService(NewImportService(mgr)),
			application.NewService(NewExportService(mgr)),
			application.NewService(NewCodegenService(mgr)),
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

	// Fan runner progress events out to every window.
	RunnerEmitter = func(name string, payload any) {
		app.Event.Emit(name, payload)
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
