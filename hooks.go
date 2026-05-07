package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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
//
// IMPORTANT (macOS): Wails calls ServiceStartup on the main OS thread, before
// NSApp's run loop is running. golang.design/x/hotkey's Register() internally
// does dispatch_sync(dispatch_get_main_queue(), ...). Calling that from the
// main thread itself is an immediate deadlock, which libdispatch traps as
// SIGTRAP (the "trace trap" / signal-during-cgo crash we used to hit here).
//
// We therefore only build the Hotkey objects in ServiceStartup and defer the
// actual Register() to events.Common.ApplicationStarted. Wails dispatches that
// callback on a regular goroutine, so the cgo dispatch_sync to the main queue
// can be serviced by the now-running main run loop.
func (s *hotkeyService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.app = application.Get()
	s.hotkeys = map[uint]*hotkey.Hotkey{
		OPEN_CLOSE: hotkey.New([]hotkey.Modifier{hotkey.ModCmd, hotkey.ModShift}, hotkey.KeyJ),
		CLOSE:      hotkey.New([]hotkey.Modifier{}, hotkey.KeyEscape),
	}

	s.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		registered := 0
		for idx, hk := range s.hotkeys {
			if err := hk.Register(); err != nil {
				log.Printf("hotkey: failed to register hotkey %d: %v", idx, err)
				continue
			}
			log.Printf("hotkey: registered %d", idx)
			registered++
		}
		if registered > 0 {
			go s.processHooks(s.app.Context())
		}
	})

	return nil
}

// targetWindow returns the window the hotkeys should act on.
//
// Do NOT use s.app.Window.Current() for global hotkeys: the trigger almost
// always happens while the app is in the background, so [NSApp keyWindow]
// is nil, getCurrentWindowID() returns 0, and Current() returns nil.
// Calling any method on that nil Window panics with SIGSEGV.
// mught need to modify this
func (s *hotkeyService) targetWindow() application.Window {
	if all := s.app.Window.GetAll(); len(all) > 0 {
		return all[0]
	}
	return nil
}

func (s *hotkeyService) processHooks(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Term signal recieved")
			return
		case <-s.hotkeys[CLOSE].Keydown():
			if w := s.targetWindow(); w != nil {
				w.Hide()
			}
		case <-s.hotkeys[OPEN_CLOSE].Keydown():
			w := s.targetWindow()

			if w == nil {
				log.Printf("hotkey: no window available to toggle")
				continue
			}
			if w.IsVisible() {
				w.Hide()
			} else {
				w.Show()
				w.Focus()
			}

		}
	}
}

// clean up the hooks
func (a *hotkeyService) ServiceShutdown() {
	for _, value := range a.hotkeys {
		value.Unregister()
	}
}
