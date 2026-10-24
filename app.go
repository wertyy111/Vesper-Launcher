package main

import (
	"context"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"vesper-launcher/internal/bridge"
	"vesper-launcher/internal/state"
)

type App struct {
	ctx    context.Context
	state  *state.LauncherState
	bridge *bridge.BridgeService
}

func NewApp() *App {
	s := state.NewLauncherState()
	b := bridge.NewBridgeService(s)
	return &App{
		state:  s,
		bridge: b,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.bridge.SetContext(ctx)

	// Listen to incoming commands from the frontend via Wails Events
	wailsRuntime.EventsOn(ctx, "vesper:command", func(optionalData ...any) {
		if len(optionalData) == 0 {
			return
		}
		if dataMap, ok := optionalData[0].(map[string]any); ok {
			cmd, _ := dataMap["command"].(string)
			payload, _ := dataMap["payload"].(map[string]any)
			_ = a.bridge.HandleCommand(cmd, payload)
		}
	})

	// Periodic heartbeat snapshot refresh (only as fallback, real events push immediately)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.bridge.EmitSnapshot()
			}
		}
	}()
}

func (a *App) domReady(ctx context.Context) {
	a.bridge.EmitSnapshot()
}

// SendCommand is exposed to the frontend JS runtime.
func (a *App) SendCommand(command string, payload map[string]any) {
	_ = a.bridge.HandleCommand(command, payload)
}

// SendMessage is exposed for raw JSON command payloads.
func (a *App) SendMessage(jsonMessage string) {
	a.bridge.ExecuteRawCommand(jsonMessage)
}
