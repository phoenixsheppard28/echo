package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AppService struct {
	app *application.App
}

func NewAppService(app *application.App) *AppService {
	return &AppService{}
}

func (a *AppService) serviceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil
}

func (a *AppService) ServiceShutdown() error {
	return nil
}
