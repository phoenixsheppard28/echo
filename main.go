package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	app := application.New(application.Options{
		Name: "echo",
		Services: []application.Service{
			application.NewService(&AppService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	_ = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:       "My App",
		Width:       1024,
		Height:      768,
		AlwaysOnTop: true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
