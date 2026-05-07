package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"
)

const (
	CLOSE      uint = 0
	OPEN_CLOSE uint = 1
)

type hotkeyService struct {
	app     *application.App
	hotkeys map[uint]*hotkey.Hotkey
}

// register all hooks
func (s *hotkeyService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	var hks []*hotkey.Hotkey

	hks = append(hks, hotkey.New([]hotkey.Modifier{}, hotkey.KeyEscape))
	hks = append(hks, hotkey.New([]hotkey.Modifier{hotkey.ModCmd, hotkey.ModShift}, hotkey.KeyJ))

	for idx, hk := range hks {
		err := hk.Register()
		if err != nil {
			log.Fatalf("hotkey: failed to register hotkey: %v", err)
			return err
		}
		s.hotkeys[uint(idx)] = hk
	}
	go s.processHooks()
	return nil
}

func (s *hotkeyService) processHooks() {
	for {
		select {
		case <-s.hotkeys[CLOSE].Keydown():
			s.app.Window.Current().Hide()

		case <-s.hotkeys[OPEN_CLOSE].Keydown():
			s.app.Window.Current().Fullscreen()
		}
	}
}

// clean up the hooks
func (a *hotkeyService) ServiceShutdown() {
	for _, value := range a.hotkeys {
		value.Unregister()
	}
}
