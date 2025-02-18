//go:build js

package main

import (
	"os"
	"syscall/js"
)

// Global variables used across different builds
var (
	// Used by WASM build to reference JavaScript game instance
	jsGameInstance string
)

func getFileSystem() js.Value {
	return js.Global().Get(jsGameInstance).Get("filesystem")
}

func getWebGLContext() js.Value {
	return js.Global().Get(jsGameInstance).Get("graphics").Get("gl")
}

func getWebAudio() js.Value {
	return js.Global().Get(jsGameInstance).Get("audio")
}

func getJSTimestamp() float64 {
	timestamp := js.Global().Get(jsGameInstance).Get("util").Call("getTimeStampSeconds")
	return float64(timestamp.Float())
}

func main() {
	// Get the game ID from environment variable instead of command line
	gameID := os.Getenv("IKEMEN_GAME_ID")
	if gameID == "" {
		panic("Game ID not provided in IKEMEN_GAME_ID environment variable")
	}

	// Set global game ID
	jsGameInstance = gameID

	cfgPath := "save/config.ini"
	if cfg, err := loadConfig(cfgPath); err != nil {
		chk(err)
	} else {
		sys.cfg = *cfg
	}
	sys.luaLState = sys.init(sys.gameWidth, sys.gameHeight)

	sys.Init()

	// Keep the program running
	c := make(chan struct{}, 0)
	<-c
}

func chk(err error) {
	if err != nil {
		ShowErrorDialog(err.Error())
		panic(err)
	}
}

// Extended version of 'chk()'
func chkEX(err error, txt string, crash bool) bool {
	if err != nil {
		ShowErrorDialog(txt + err.Error())
		if crash {
			panic(Error(txt + err.Error()))
		}
		return true
	}
	return false
}
