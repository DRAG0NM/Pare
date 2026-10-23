// Command pare-desktop is the Pare desktop UI. It only assembles the core packages and
// forwards progress to the web frontend; rules and safety decisions stay in the core.
//
// Build the frontend first (pnpm --dir desktop/frontend build), then:
//
//	go build -tags desktop,production -ldflags "-H windowsgui" -o pare-desktop.exe ./desktop
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:       "Pare",
		Width:       1280,
		Height:      820,
		MinWidth:    960,
		MinHeight:   600,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		Bind:        []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
