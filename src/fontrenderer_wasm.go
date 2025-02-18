//go:build js

package main

import (
	"fmt"
	"image"
	"image/draw"

	"io"
	"syscall/js"
	"unsafe"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// FontRendererWrapper wraps glfont.FontRenderer types to implement the local FontRenderer interface
type FontRendererWeb struct{}

// FontWrapper wraps glfont.Font to implement the local Font interface
type FontWeb struct {
	fontChar map[rune]*character
	ttf      *truetype.Font
	scale    int32
	vao      js.Value
	vbo      js.Value
	program  js.Value
	texture  js.Value // Holds the glyph texture id.
	color    color
}

type character struct {
	texture  js.Value // glyph texture
	width    int      //glyph width
	height   int      //glyph height
	advance  int      //glyph advance
	bearingH int      //glyph bearing horizontal
	bearingV int      //glyph bearing vertical
}

type color struct {
	r float32
	g float32
	b float32
	a float32
}

func (f *FontWeb) SetColor(r, g, b, a float32) {
	f.color.r = r
	f.color.g = g
	f.color.b = b
	f.color.a = a
}
func (f *FontWeb) UpdateResolution(windowWidth, windowHeight int) {
	webGL := getWebGLContext()

	webGL.Call("useProgram", f.program)
	resUniform := webGL.Call("getUniformLocation", f.program, "resolution")
	webGL.Call("uniform2f", resUniform, float32(windowWidth), float32(windowHeight))
	webGL.Call("useProgram", js.Null())
}

func (f *FontWeb) Printf(x, y, scale float32, align int32, blend bool, window [4]int32, fs string, argv ...interface{}) error {
	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return nil
	}

	webGL := getWebGLContext()

	// Buffer to store vertex data for multiple glyphs
	var batchVertices []float32
	var batchChars []*character
	//setup blending mode
	if blend {
		webGL.Call("enable", webGL.Get("BLEND"))
		webGL.Call("blendFunc",
			webGL.Get("SRC_ALPHA"),
			webGL.Get("ONE_MINUS_SRC_ALPHA"),
		)
	}

	//restrict drawing to a certain part of the window
	webGL.Call("enable", webGL.Get("SCISSOR_TEST"))
	webGL.Call("scissor",
		window[0], window[1],
		window[2], window[3],
	)

	// Activate corresponding render state
	webGL.Call("useProgram", f.program)
	//set text color
	webGL.Call("uniform4f",
		webGL.Call("getUniformLocation", f.program, "textColor"),
		f.color.r, f.color.g, f.color.b, f.color.a,
	)

	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))
	webGL.Call("bindVertexArray", f.vao)

	//calculate alignment position
	if align == 0 {
		x -= f.Width(scale, fs, argv...) * 0.5
	} else if align < 0 {
		x -= f.Width(scale, fs, argv...)
	}

	// Iterate through all characters in string
	for i := range indices {

		//get rune
		runeIndex := indices[i]

		//find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		//load missing runes in batches of 32
		if !ok {
			low := runeIndex - (runeIndex % 32)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		//skip runes that are not in font chacter range
		if !ok {
			//fmt.Printf("%c %d\n", runeIndex, runeIndex)
			continue
		}

		//calculate position and size for current rune
		xpos := x + float32(ch.bearingH)*scale
		ypos := y - float32(ch.height-ch.bearingV)*scale
		w := float32(ch.width) * scale
		h := float32(ch.height) * scale
		vertices := []float32{
			xpos + w, ypos, 1.0, 0.0,
			xpos, ypos, 0.0, 0.0,
			xpos, ypos + h, 0.0, 1.0,

			xpos, ypos + h, 0.0, 1.0,
			xpos + w, ypos + h, 1.0, 1.0,
			xpos + w, ypos, 1.0, 0.0,
		}
		// Append glyph vertices to the batch buffer
		batchVertices = append(batchVertices, vertices...)
		batchChars = append(batchChars, ch)

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		x += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))
	}

	// Render any remaining glyphs in the batch
	if len(batchVertices) > 0 {
		f.renderGlyphBatch(batchChars, indices, batchVertices)
	}

	//clear opengl textures and programs
	webGL.Call("bindVertexArray", js.Null())
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), js.Null())
	webGL.Call("useProgram", js.Null())
	if blend {
		webGL.Call("disable", webGL.Get("BLEND"))
	}
	webGL.Call("disable", webGL.Get("SCISSOR_TEST"))

	return nil
}

func (f *FontWeb) renderGlyphBatch(batchChars []*character, indices []rune, vertices []float32) {
	webGL := getWebGLContext()

	verticesArray := js.Global().Get("Float32Array").New(len(vertices))
	data := unsafe.Slice(
		(*byte)(unsafe.Pointer(&vertices[0])), len(vertices)*4,
	)
	js.CopyBytesToJS(verticesArray, data)

	// Bind the buffer and update its data
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), f.vbo)
	webGL.Call("bufferData",
		webGL.Get("ARRAY_BUFFER"),
		verticesArray,
		webGL.Get("DYNAMIC_DRAW"),
	)
	// Iterate over each glyph in the batch
	for i := 0; i < len(vertices)/24; i++ {

		// Bind the texture
		webGL.Call("bindTexture",
			webGL.Get("TEXTURE_2D"),
			batchChars[i].texture,
		)

		// Render the current glyph
		webGL.Call("drawArrays",
			webGL.Get("TRIANGLES"), i*6, 6,
		)
	}

	// Unbind the buffer and texture
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), js.Null())
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), js.Null())
}

func (f *FontWeb) Width(scale float32, fs string, argv ...interface{}) float32 {

	var width float32

	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return 0
	}

	// Iterate through all characters in string
	for i := range indices {

		//get rune
		runeIndex := indices[i]

		//find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		//load missing runes in batches of 32
		if !ok {
			low := runeIndex & rune(32-1)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		//skip runes that are not in font chacter range
		if !ok {
			//fmt.Printf("%c %d\n", runeIndex, runeIndex)
			continue
		}

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		width += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))

	}

	return width
}

// GenerateGlyphs builds a set of textures based on a ttf files gylphs
func (f *FontWeb) GenerateGlyphs(low, high rune) error {
	//create a freetype context for drawing
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f.ttf)
	c.SetFontSize(float64(f.scale))
	c.SetHinting(font.HintingFull)

	//create new face to measure glyph dimensions
	ttfFace := truetype.NewFace(f.ttf, &truetype.Options{
		Size:    float64(f.scale),
		DPI:     72,
		Hinting: font.HintingFull,
	})

	webGL := getWebGLContext()

	//make each gylph
	for ch := low; ch <= high; ch++ {
		char := new(character)

		gBnd, gAdv, ok := ttfFace.GlyphBounds(ch)
		if ok != true {
			return fmt.Errorf("ttf face glyphBounds error")
		}

		gh := int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)
		gw := int32((gBnd.Max.X - gBnd.Min.X) >> 6)

		//if gylph has no dimensions set to a max value
		if gw == 0 || gh == 0 {
			gBnd = f.ttf.Bounds(fixed.Int26_6(f.scale))
			gw = int32((gBnd.Max.X - gBnd.Min.X) >> 6)
			gh = int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)

			//above can sometimes yield 0 for font smaller than 48pt, 1 is minimum
			if gw == 0 || gh == 0 {
				gw = 1
				gh = 1
			}
		}

		//The glyph's ascent and descent equal -bounds.Min.Y and +bounds.Max.Y.
		gAscent := int(-gBnd.Min.Y) >> 6
		gdescent := int(gBnd.Max.Y) >> 6

		//set w,h and adv, bearing V and bearing H in char
		char.width = int(gw)
		char.height = int(gh)
		char.advance = int(gAdv)
		char.bearingV = gdescent
		char.bearingH = (int(gBnd.Min.X) >> 6)

		//create image to draw glyph
		fg, bg := image.White, image.Black
		rect := image.Rect(0, 0, int(gw), int(gh))
		rgba := image.NewRGBA(rect)
		draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)

		//set the glyph dot
		px := 0 - (int(gBnd.Min.X) >> 6)
		py := (gAscent)
		pt := freetype.Pt(px, py)

		// Draw the text from mask to image
		c.SetClip(rgba.Bounds())
		c.SetDst(rgba)
		c.SetSrc(fg)
		_, err := c.DrawString(string(ch), pt)
		if err != nil {
			return err
		}

		// Generate texture

		// Generate texture
		texture := webGL.Call("createTexture")
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), texture)

		// Set texture parameters
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_2D"),
			webGL.Get("TEXTURE_WRAP_S"),
			webGL.Get("CLAMP_TO_EDGE"),
		)
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_2D"),
			webGL.Get("TEXTURE_WRAP_T"),
			webGL.Get("CLAMP_TO_EDGE"),
		)
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_2D"),
			webGL.Get("TEXTURE_MIN_FILTER"),
			webGL.Get("LINEAR"),
		)
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_2D"),
			webGL.Get("TEXTURE_MAG_FILTER"),
			webGL.Get("LINEAR"),
		)

		// Create TypedArray from image data
		pixels := unsafe.Slice((*byte)(unsafe.Pointer(&rgba.Pix[0])), len(rgba.Pix))
		jsPixels := js.Global().Get("Uint8Array").New(len(rgba.Pix))
		js.CopyBytesToJS(jsPixels, pixels)

		// Upload the image data to texture
		webGL.Call("texImage2D",
			webGL.Get("TEXTURE_2D"),
			0,                          // level
			webGL.Get("RGBA"),          // internalFormat
			rgba.Rect.Dx(),             // width
			rgba.Rect.Dy(),             // height
			0,                          // border
			webGL.Get("RGBA"),          // format
			webGL.Get("UNSIGNED_BYTE"), // type
			jsPixels,                   // pixels
		)

		// Store texture reference
		char.texture = texture // Store WebGL texture

		// Add char to fontChar list
		f.fontChar[ch] = char
	}

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), js.Null())
	return nil
}

// Implement FontRenderer interface methods
func (fr *FontRendererWeb) LoadFont(file string, scale int32, windowWidth int, windowHeight int) (Font, error) {
	fd, err := fs.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	webGL := getWebGLContext()

	// Configure the default font vertex and fragment shaders
	vertexShader := webGL.Call("createShader", webGL.Get("VERTEX_SHADER"))
	webGL.Call("shaderSource", vertexShader, vertexFontShader)
	webGL.Call("compileShader", vertexShader)
	if err := checkShaderError(webGL, vertexShader); err != nil {
		return nil, fmt.Errorf("vertex shader error: %v", err)
	}
	fragmentShader := webGL.Call("createShader", webGL.Get("FRAGMENT_SHADER"))
	webGL.Call("shaderSource", fragmentShader, fragmentFontShader)
	webGL.Call("compileShader", fragmentShader)
	if err := checkShaderError(webGL, fragmentShader); err != nil {
		return nil, fmt.Errorf("vertex shader error: %v", err)
	}
	// Create and link program
	program := webGL.Call("createProgram")
	webGL.Call("attachShader", program, vertexShader)
	webGL.Call("attachShader", program, fragmentShader)
	webGL.Call("linkProgram", program)
	if err := checkProgramError(webGL, program); err != nil {
		return nil, fmt.Errorf("program link error: %v", err)
	}

	// Cleanup shaders after linking
	webGL.Call("deleteShader", vertexShader)
	webGL.Call("deleteShader", fragmentShader)

	// Activate corresponding render state
	webGL.Call("useProgram", program)

	//set screen resolution
	resUniform := webGL.Call("getUniformLocation", program, "resolution")
	webGL.Call("uniform2f", resUniform, float32(windowWidth), float32(windowHeight))

	return fr.LoadTrueTypeFont(program, fd, scale, 32, 127, LeftToRight)
}

func (fr *FontRendererWeb) LoadTrueTypeFont(program interface{}, r io.Reader, scale int32, low, high rune, dir Direction) (Font, error) {
	jsProgram, ok := program.(js.Value)
	if !ok {
		return nil, Error("program must be js.Value in WASM build")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Read the truetype font.
	ttf, err := truetype.Parse(data)
	if err != nil {
		return nil, err
	}

	//make Font stuct type
	f := &FontWeb{
		fontChar: make(map[rune]*character),
		ttf:      ttf,
		scale:    scale,
		program:  jsProgram,
	}
	f.SetColor(1.0, 1.0, 1.0, 1.0) //set default white

	err = f.GenerateGlyphs(low, high)
	if err != nil {
		return nil, err
	}

	webGL := getWebGLContext()

	// Generate the VAO and VBO
	f.vao = webGL.Call("createVertexArray")
	f.vbo = webGL.Call("createBuffer")
	webGL.Call("bindVertexArray", f.vao)
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), f.vbo)

	// 6 vertices * 4 components * 4 bytes
	buffer := js.Global().Get("ArrayBuffer").New(6 * 4 * 4)

	webGL.Call("bufferData",
		webGL.Get("ARRAY_BUFFER"),
		buffer,
		webGL.Get("STATIC_DRAW"),
	)

	vertAttrib := webGL.Call("getAttribLocation", f.program, "vert")
	webGL.Call("enableVertexAttribArray", vertAttrib)
	webGL.Call("vertexAttribPointer",
		vertAttrib,
		2, // size (vec2)
		webGL.Get("FLOAT"),
		false, // normalized
		4*4,   // stride (4 components * 4 bytes)
		0,
	)

	texCoordAttrib := webGL.Call("getAttribLocation", f.program, "vertTexCoord")
	webGL.Call("enableVertexAttribArray", texCoordAttrib)
	webGL.Call("vertexAttribPointer",
		texCoordAttrib,
		2, // size (vec2)
		webGL.Get("FLOAT"),
		false, // normalized
		4*4,   // stride (4 components * 4 bytes)
		2*4,   // offset (2 components * 4 bytes)
	)

	// cleanup
	webGL.Call("disableVertexAttribArray", vertAttrib)
	webGL.Call("disableVertexAttribArray", texCoordAttrib)
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), js.Null())
	webGL.Call("bindVertexArray", js.Null())

	return f, nil
}

func NewFontRenderer(config Config) FontRenderer {
	fr := &FontRendererWeb{}

	webGL := getWebGLContext()
	vaoExt := webGL.Call("getExtension", "OES_vertex_array_object")
	if vaoExt.IsNull() {
		panic("WebGL VAO extension not supported")
	}
	return fr
}

func checkShaderError(webGL js.Value, shader js.Value) error {
	if !webGL.Call("getShaderParameter", shader, webGL.Get("COMPILE_STATUS")).Bool() {
		info := webGL.Call("getShaderInfoLog", shader).String()
		return fmt.Errorf("shader compilation failed: %s", info)
	}
	return nil
}

func checkProgramError(webGL js.Value, program js.Value) error {
	if !webGL.Call("getProgramParameter", program, webGL.Get("LINK_STATUS")).Bool() {
		info := webGL.Call("getProgramInfoLog", program).String()
		return fmt.Errorf("program linking failed: %s", info)
	}
	return nil
}

var fragmentFontShader = `
precision mediump float;
varying vec2 fragTexCoord;
uniform sampler2D tex;
uniform vec4 textColor;

void main() {
    vec4 sampled = vec4(1.0, 1.0, 1.0, texture2D(tex, fragTexCoord).r);
    gl_FragColor = min(textColor, vec4(1.0, 1.0, 1.0, 1.0)) * sampled;
}`

var vertexFontShader = `
attribute vec2 vert;
attribute vec2 vertTexCoord;
uniform vec2 resolution;
varying vec2 fragTexCoord;

void main() {
    vec2 zeroToOne = vert / resolution;
    vec2 zeroToTwo = zeroToOne * 2.0;
    vec2 clipSpace = zeroToTwo - 1.0;
    fragTexCoord = vertTexCoord;
    gl_Position = vec4(clipSpace * vec2(1, -1), 0, 1);
}`
