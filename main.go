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
			application.NewService(&hotkeyService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	// register keybindings for the app at the global level, shoud use a seperate service liek gohook, since t

	_ = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:       "My App",
		Width:       500,
		Height:      300,
		AlwaysOnTop: true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

}
