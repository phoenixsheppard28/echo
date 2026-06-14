package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	launcherWindowWidth  = 760
	launcherWindowHeight = 96
	chatWindowWidth      = 920
	chatWindowHeight     = 680
)

type AppService struct {
	app      *application.App
	calendar *GoogleCalendarClient
	ollama   *OllamaClient
}

func NewAppService(app *application.App) *AppService {
	return &AppService{}
}

func (s *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.app = application.Get()
	s.calendar = NewGoogleCalendarClient()
	s.ollama = NewOllamaClient("llama3")
	return nil
}

func (s *AppService) ServiceShutdown() error {
	return nil
}

func (s *AppService) CalendarStatus() CalendarStatus {
	return s.calendarClient().Status()
}

func (s *AppService) ConnectGoogleCalendar() CalendarStatus {
	return s.calendarClient().Connect()
}

func (s *AppService) Chat(req ChatRequest) ChatResponse {
	return s.assistant().Chat(req)
}

func (s *AppService) SetWindowMode(mode string) WindowModeResponse {
	width, height := launcherWindowWidth, launcherWindowHeight
	if mode == "chat" {
		width, height = chatWindowWidth, chatWindowHeight
	}

	if w := firstWindow(s.app); w != nil {
		w.SetSize(width, height)
		w.Center()
		w.Focus()
	}

	return WindowModeResponse{Mode: mode, Width: width, Height: height}
}

func (s *AppService) HideWindow() {
	if w := firstWindow(s.app); w != nil {
		w.Hide()
	}
}

func (s *AppService) calendarClient() *GoogleCalendarClient {
	if s.calendar == nil {
		s.calendar = NewGoogleCalendarClient()
	}
	return s.calendar
}

func (s *AppService) assistant() *Assistant {
	return &Assistant{
		calendar: s.calendarClient(),
		ollama:   s.ollamaClient(),
	}
}

func (s *AppService) ollamaClient() *OllamaClient {
	if s.ollama == nil {
		s.ollama = NewOllamaClient("llama3")
	}
	return s.ollama
}

func firstWindow(app *application.App) application.Window {
	if app == nil {
		app = application.Get()
	}
	if app == nil {
		return nil
	}
	if all := app.Window.GetAll(); len(all) > 0 {
		return all[0]
	}
	return nil
}
