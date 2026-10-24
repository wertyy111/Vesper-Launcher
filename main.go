package main

/*
#include <stdlib.h>

__attribute__((constructor)) static void setup_linux_env() {
    const char *display = getenv("DISPLAY");
    if (display != NULL && display[0] != '\0') {
        setenv("GDK_BACKEND", "x11", 1);
    }
    setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 1);
    setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1", 1);
}
*/
import "C"

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"vesper-launcher/internal/state"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// Frameless window across platforms to remove native window decoration notch ("челка") and match Windows
	isFrameless := true

	width := app.state.WindowWidth
	if width < state.MinWindowWidth || width > state.MaxWindowWidth {
		width = state.DefaultWindowWidth
	}
	height := app.state.WindowHeight
	if height < state.MinWindowHeight || height > state.MaxWindowHeight {
		height = state.DefaultWindowHeight
	}

	err := wails.Run(&options.App{
		Title:             "Vesper Launcher",
		Width:             width,
		Height:            height,
		MinWidth:          state.MinWindowWidth,
		MinHeight:         state.MinWindowHeight,
		MaxWidth:          state.MaxWindowWidth,
		MaxHeight:         state.MaxWindowHeight,
		WindowStartState:  options.Normal,
		DisableResize:     false,
		Frameless:         isFrameless,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 10, G: 17, B: 24, A: 255},
		LogLevel:          logger.ERROR,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		Bind: []any{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.None,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarDefault(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
			DisableZoom:          true,
		},
		Linux: &linux.Options{
			Icon:                nil,
			WindowIsTranslucent: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
