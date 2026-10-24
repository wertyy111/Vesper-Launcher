package main

/*
#include <stdlib.h>

__attribute__((constructor)) static void setup_linux_env() {
    setenv("GDK_BACKEND", "x11", 0);
    setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 0);
    setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1", 0);
}
*/
import "C"

import (
	"embed"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// Frameless only on Windows (Linux and macOS use standard native OS window frame)
	isFrameless := runtime.GOOS == "windows"

	width := app.state.WindowWidth
	if width < 900 {
		width = 1020
	}
	height := app.state.WindowHeight
	if height < 600 {
		height = 640
	}

	err := wails.Run(&options.App{
		Title:             "Vesper Launcher",
		Width:             width,
		Height:            height,
		MinWidth:          900,
		MinHeight:         620,
		Frameless:         isFrameless,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 10, G: 17, B: 24, A: 255},
		LogLevel:          logger.DEBUG,
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
