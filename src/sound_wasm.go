//go:build js

package main

import (
	"sync"
	"syscall/js"
)

type WebAudioSystem struct {
	jsGameID string
	mutex    sync.Mutex
}

func New(gameID string) *WebAudioSystem {
	return &WebAudioSystem{
		jsGameID: gameID,
	}
}

func (w *WebAudioSystem) getAudioContext() js.Value {
	return getWebAudio()
}

func (w *WebAudioSystem) Init(sampleRate int, bufferSize int) error {
	// Initialize is handled by JavaScript
	return nil
}

type WebAudioStreamer struct {
	jsGameID string
	sourceID string
	buffer   js.Value
}

func (w *WebAudioStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	// Convert samples to JavaScript Float32Array and send to audio worklet
	audioData := js.Global().Get("Float32Array").New(len(samples) * 2)
	for i, sample := range samples {
		audioData.SetIndex(i*2, sample[0])
		audioData.SetIndex(i*2+1, sample[1])
	}

	js.Global().Get(w.jsGameID).Get("audio").Call("streamAudio", w.sourceID, audioData)
	return len(samples), true
}

type WebBGMPlayer struct {
	jsGameID string
	sourceID string
}

func (b *WebBGMPlayer) Open(filename string, loop int, volume int, loopStart, loopEnd, startPosition int, freqMul float32) error {
	js.Global().Get(b.jsGameID).Get("audio").Call("playBGM", map[string]interface{}{
		"filename":      filename,
		"loop":          loop,
		"volume":        volume,
		"loopStart":     loopStart,
		"loopEnd":       loopEnd,
		"startPosition": startPosition,
		"freqMul":       freqMul,
	})
	return nil
}

func (b *WebBGMPlayer) SetPaused(pause bool) {
	js.Global().Get(b.jsGameID).Get("audio").Call("setBGMPaused", pause)
}

func (b *WebBGMPlayer) SetVolume(volume int) {
	js.Global().Get(b.jsGameID).Get("audio").Call("setBGMVolume", volume)
}

type WebSoundChannel struct {
	jsGameID  string
	channelID string
}

func (s *WebSoundChannel) Play(sound StreamSeeker, group, number int32, loop int32, freqMul float32, loopStart, loopEnd, startPosition int) {
	js.Global().Get(s.jsGameID).Get("audio").Call("playSoundChannel", map[string]interface{}{
		"channelID":     s.channelID,
		"soundID":       sound.(*WebAudioStreamer).sourceID,
		"group":         group,
		"number":        number,
		"loop":          loop,
		"freqMul":       freqMul,
		"loopStart":     loopStart,
		"loopEnd":       loopEnd,
		"startPosition": startPosition,
	})
}

func (s *WebSoundChannel) SetPan(pan, ls float32, x *float32) {
	var xVal float32
	if x != nil {
		xVal = *x
	}
	js.Global().Get(s.jsGameID).Get("audio").Call("setSoundPan", s.channelID, pan, ls, xVal)
}
