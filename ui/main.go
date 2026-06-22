// Command migrator-ui is the Wails desktop application. It binds the App object
// (a thin layer over internal/app.Service) to a Svelte frontend.
//
// Build/run with the Wails toolchain (not plain `go build`):
//
//	wails dev      # hot-reload development
//	wails build    # produce a native binary for the current OS
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := NewApp()
	err := wails.Run(&options.App{
		Title:     "migrator — DB Migration",
		Width:     1100,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: a.startup,
		Bind:      []interface{}{a},
	})
	if err != nil {
		panic(err)
	}
}
