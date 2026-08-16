package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var iconBytes []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "App Cleaner",
		Width:     1150,
		Height:    740,
		MinWidth:  940,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.guilhermevozniak.appcleaner",
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "App Cleaner",
				Message: "Version " + appVersion + "\n© 2026 Guilherme Vozniak — MIT\n\nA macOS cleaning app — Go/Wails port of mac-cleaner-cli.",
				Icon:    iconBytes,
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
