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

func (s *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil
}

func (s *AppService) ServiceShutdown() error {
	return nil
}
