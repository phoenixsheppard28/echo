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
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory, // removes it from dock
		},
	})

	_ = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Echo",
		BackgroundType: application.BackgroundTypeTranslucent,
		Width:          launcherWindowWidth,
		Height:         launcherWindowHeight,
		MinWidth:       launcherWindowWidth,
		MinHeight:      launcherWindowHeight,
		AlwaysOnTop:    true,
		Frameless:      true,
		Hidden:         true,

		Mac: application.MacWindow{
			WindowLevel:             application.MacWindowLevelFloating,
			TitleBar:                application.MacTitleBarHidden,
			Backdrop:                application.MacBackdropTranslucent,
			NonActivatingPanel:      true,
			InvisibleTitleBarHeight: 50,
		},

		HideOnFocusLost: true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

}
