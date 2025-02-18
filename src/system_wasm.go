//go:build js

package main

import (
	"image"
	"syscall/js"

	lua "github.com/yuin/gopher-lua"
)

type Window struct {
	gameID     string
	width      int
	height     int
	fullscreen bool
}

func (s *System) newWindow(w, h int) (*Window, error) {
	window := &Window{
		gameID:     jsGameInstance,
		width:      w,
		height:     h,
		fullscreen: false,
	}

	js.Global().Get(jsGameInstance).Get("graphics").Call(
		"fullScreenEvent", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if len(args) < 1 {
				return nil
			}
			window.fullscreen = args[0].Bool()
			return nil
		}),
	)

	return window, nil
}

func (w *Window) SwapBuffers() {
	js.Global().Get(w.gameID).Get("renderer").Call("swapBuffers")
}

func (w *Window) GetSize() (int, int) {
	return w.width, w.height
}

func (w *Window) GetScaledViewportSize() (int32, int32, int32, int32) {
	// Get viewport size from JavaScript
	viewport := js.Global().Get(w.gameID).Get("renderer").Call("getViewport")
	return int32(0),
		int32(0),
		int32(viewport.Get("width").Int()),
		int32(viewport.Get("height").Int())
}

func (w *Window) GetClipboardString() string {
	// Use browser clipboard API if needed
	wait := make(chan interface{})
	promise := js.Global().Get(w.gameID).Get("util").Call("getClipboardText")
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		wait <- nil
		return nil
	}))
	<-wait
	return ""
}

func (w *Window) toggleFullscreen() {
	js.Global().Get(w.gameID).Get("renderer").Call("toggleFullscreen")
}

func (w *Window) pollEvents() {
	// Events are handled by JavaScript
}

func (w *Window) shouldClose() bool {
	return false // Handled by JavaScript
}

func (w *Window) Close() {
	// Cleanup if needed
}

func (s *System) Init() *lua.LState {

	// Initialize window size
	s.setWindowSize(320, 240) // Default size

	var err error
	s.window, err = s.newWindow(int(s.scrrect[2]), int(s.scrrect[3]))
	if err != nil {
		panic(err)
	}

	// Initialize audio
	// TODO: Web audio implementation

	// Initialize Lua
	l := lua.NewState()
	l.Options.IncludeGoStackTrace = true
	l.OpenLibs()

	// Initialize other systems
	for i := range s.inputRemap {
		s.inputRemap[i] = i
	}

	systemScriptInit(l)
	s.shortcutScripts = make(map[ShortcutKey]*ShortcutScript)

	return l
}

func (w *Window) SetIcon(icons []image.Image) {
	// No-op for web
}

func (w *Window) SetSwapInterval(interval int) {
	// No-op for web - vsync handled by browser
}
