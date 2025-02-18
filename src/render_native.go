//go:build !js

package main

func NewRenderer(config Config) Renderer {
	if config.Video.RenderMode == "OpenGL 2.1" {
		return &Renderer_GL21{}
	} else {
		return &Renderer_GL32{}
	}
}
