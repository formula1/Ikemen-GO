package main

import (
	"io"
)

type FontRenderer interface {
	LoadFont(file string, scale int32, windowWidth int, windowHeight int) (Font, error)
	LoadTrueTypeFont(program interface{}, r io.Reader, scale int32, low, high rune, dir Direction) (Font, error)
}

type Font interface {
	SetColor(red float32, green float32, blue float32, alpha float32)
	UpdateResolution(windowWidth int, windowHeight int)
	Printf(x, y float32, scale float32, align int32, blend bool, window [4]int32, fs string, argv ...interface{}) error
	Width(scale float32, fs string, argv ...interface{}) float32
}

// Direction represents the direction in which strings should be rendered.
type Direction uint8

const (
	LeftToRight Direction = iota // E.g.: Latin
	RightToLeft                  // E.g.: Arabic
	TopToBottom                  // E.g.: Chinese
)
