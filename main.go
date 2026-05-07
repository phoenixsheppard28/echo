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
		Title:          "Echo",
		BackgroundType: application.BackgroundTypeTranslucent,
		Width:          500,
		Height:         300,
		AlwaysOnTop:    true,
		Frameless:      true,

		Mac: application.MacWindow{
			WindowLevel:             application.MacWindowLevelFloating,
			TitleBar:                application.MacTitleBarHidden,
			Backdrop:                application.MacBackdropTranslucent,
			NonActivatingPanel:      true, // I DID THIS MYSELF LETS GOOOOOOO
			InvisibleTitleBarHeight: 50,
		},

		HideOnFocusLost: true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

}
