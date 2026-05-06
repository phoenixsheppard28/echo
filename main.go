package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appService := NewApp()

	app := application.New(application.Options{
		Name: "echo",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	appService.app = app

	_ = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "My App",
		Width:  1024,
		Height: 768,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
