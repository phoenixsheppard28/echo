package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	app     *application.App
	visible bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.visible = false
	return nil
}

func (a *App) ServiceShutdown() error {
	return nil
}
