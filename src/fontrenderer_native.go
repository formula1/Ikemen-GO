//go:build !js

package main

import (
	"io"

	"github.com/ikemen-engine/glfont"
)

// FontRendererWrapper wraps glfont.FontRenderer types to implement the local FontRenderer interface
type FontRendererWrapper struct {
	renderer interface {
		LoadFont(file string, scale int32, windowWidth int, windowHeight int) (glfont.Font, error)
		LoadTrueTypeFont(program uint32, r io.Reader, scale int32, low, high rune, dir glfont.Direction) (glfont.Font, error)
	}
}

// FontWrapper wraps glfont.Font to implement the local Font interface
type FontWrapper struct {
	font glfont.Font
}

func (w *FontWrapper) SetColor(r, g, b, a float32) {
	w.font.SetColor(r, g, b, a)
}
func (w *FontWrapper) UpdateResolution(width, height int) {
	w.font.UpdateResolution(width, height)
}
func (w *FontWrapper) Printf(x, y, scale float32, align int32, blend bool, window [4]int32, fs string, argv ...interface{}) error {
	return w.font.Printf(x, y, scale, align, blend, window, fs, argv...)
}

func (w *FontWrapper) Width(scale float32, fs string, argv ...interface{}) float32 {
	return w.font.Width(scale, fs, argv...)
}

// Implement FontRenderer interface methods
func (w *FontRendererWrapper) LoadFont(file string, scale int32, windowWidth int, windowHeight int) (Font, error) {
	f, err := w.renderer.LoadFont(file, scale, windowWidth, windowHeight)
	if err != nil {
		return nil, err
	}
	return &FontWrapper{font: f}, nil
}

func (w *FontRendererWrapper) LoadTrueTypeFont(program interface{}, r io.Reader, scale int32, low, high rune, dir Direction) (Font, error) {
	// Convert our Direction type to glfont.Direction
	programUint, ok := program.(uint32)
	if !ok {
		return nil, Error("program must be uint32 in native build")
	}
	glDir := glfont.Direction(dir)
	f, err := w.renderer.LoadTrueTypeFont(programUint, r, scale, low, high, glDir)
	if err != nil {
		return nil, err
	}
	return &FontWrapper{font: f}, nil
}

func NewFontRenderer(config Config) FontRenderer {
	if config.Video.RenderMode == "OpenGL 2.1" {
		return &FontRendererWrapper{renderer: &glfont.FontRenderer_GL21{}}
	} else {
		return &FontRendererWrapper{renderer: &glfont.FontRenderer_GL32{}}
	}
}
