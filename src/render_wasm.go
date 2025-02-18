//go:build !kinc && js

package main

import (
	_ "embed" // Support for go:embed resources
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall/js"
	"unsafe"

	mgl "github.com/go-gl/mathgl/mgl32"
	"golang.org/x/mobile/exp/f32"
)

func NewRenderer(config Config) Renderer {
	return &RendererWeb{}
}

// ------------------------------------------------------------------
// RendererWeb

type RendererWeb struct {
	fbo         js.Value
	fbo_texture js.Value
	// Normal rendering
	rbo_depth js.Value
	// MSAA rendering
	fbo_f         js.Value
	fbo_f_texture *TextureWeb
	// Shadow Map
	fbo_shadow              js.Value
	fbo_shadow_cube_texture [4]js.Value
	fbo_env                 js.Value
	// Postprocessing FBOs
	fbo_pp         []js.Value
	fbo_pp_texture []js.Value
	// Post-processing shaders
	postVertBuffer   js.Value
	postShaderSelect []*ShaderProgramWeb
	// Shader and vertex data for primitive rendering
	spriteShader *ShaderProgramWeb
	vertexBuffer js.Value
	// Shader and index data for 3D model rendering
	shadowMapShader         *ShaderProgramWeb
	modelShader             *ShaderProgramWeb
	panoramaToCubeMapShader *ShaderProgramWeb
	cubemapFilteringShader  *ShaderProgramWeb
	stageVertexBuffer       js.Value
	stageIndexBuffer        js.Value

	enableModel  bool
	enableShadow bool
}

// Render initialization.
// Creates the default shaders, the framebuffer and enables MSAA.
func (r *RendererWeb) Init() {
	webGL := getWebGLContext()

	r.enableModel = sys.cfg.Video.EnableModel
	r.enableShadow = sys.cfg.Video.EnableModelShadow

	maxSamples := int32(webGL.Call("getParameter", webGL.Get("MAX_SAMPLES")).Int())
	if sys.msaa > maxSamples {
		sys.cfg.SetValueUpdate("Video.MSAA", maxSamples)
		sys.msaa = maxSamples
	}

	// Store current timestamp
	sys.prevTimestamp = getJSTimestamp()

	r.postShaderSelect = make([]*ShaderProgramWeb, 1+len(sys.cfg.Video.ExternalShaders))

	// Data buffers for rendering
	postVertData := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)

	// Create buffer for post vertex data
	r.postVertBuffer = webGL.Call("createBuffer")
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.postVertBuffer)

	// Create TypedArray from vertex data
	jsPostVertData := js.Global().Get("Float32Array").New(len(postVertData))
	js.CopyBytesToJS(jsPostVertData, postVertData)
	webGL.Call("bufferData",
		webGL.Get("ARRAY_BUFFER"),
		jsPostVertData,
		webGL.Get("STATIC_DRAW"),
	)

	// Create other buffers
	r.vertexBuffer = webGL.Call("createBuffer")
	r.stageVertexBuffer = webGL.Call("createBuffer")
	r.stageIndexBuffer = webGL.Call("createBuffer")

	// Sprite shader
	r.spriteShader, _ = r.newShaderProgram(vertShader, fragShader, "", "Main Shader", true)
	r.spriteShader.RegisterAttributes("position", "uv")
	r.spriteShader.RegisterUniforms(
		"modelview", "projection", "x1x2x4x3",
		"alpha", "tint", "mask", "neg", "gray", "add", "mult", "isFlat", "isRgba", "isTrapez", "hue",
	)
	r.spriteShader.RegisterTextures("pal", "tex")

	if r.enableModel {
		if err := r.InitModelShader(); err != nil {
			r.enableModel = false
		}
	}

	// Compile postprocessing shaders

	// Calculate total amount of shaders loaded.
	r.postShaderSelect = make([]*ShaderProgramWeb, 1+len(sys.cfg.Video.ExternalShaders))

	// Ident shader (no postprocessing)
	r.postShaderSelect[0], _ = r.newShaderProgram(identVertShader, identFragShader, "", "Identity Postprocess", true)
	r.postShaderSelect[0].RegisterAttributes("VertCoord")
	r.postShaderSelect[0].RegisterUniforms("Texture_GL21", "TextureSize", "CurrentTime")

	// External Shaders
	for i := 0; i < len(sys.cfg.Video.ExternalShaders); i++ {
		r.postShaderSelect[1+i], _ = r.newShaderProgram(
			sys.externalShaders[0][i],
			sys.externalShaders[1][i],
			"",
			fmt.Sprintf("Postprocess Shader #%v", i+1),
			true,
		)
		r.postShaderSelect[1+i].RegisterUniforms("Texture_GL21", "TextureSize", "CurrentTime")
	}

	if sys.msaa > 0 {
		webGL.Call("enable", webGL.Get("MULTISAMPLE"))
	}

	r.fbo_texture = webGL.Call("createTexture")
	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))

	if sys.msaa > 0 {
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D_MULTISAMPLE"), r.fbo_texture)
	} else {
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), r.fbo_texture)
	}

	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("NEAREST"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), webGL.Get("NEAREST"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))

	if sys.msaa > 0 {
		webGL.Call("texImage2DMultisample",
			webGL.Get("TEXTURE_2D_MULTISAMPLE"),
			sys.msaa,
			webGL.Get("RGBA"),
			sys.scrrect[2],
			sys.scrrect[3],
			true,
		)
	} else {
		webGL.Call("texImage2D",
			webGL.Get("TEXTURE_2D"),
			0,
			webGL.Get("RGBA"),
			sys.scrrect[2],
			sys.scrrect[3],
			0,
			webGL.Get("RGBA"),
			webGL.Get("UNSIGNED_BYTE"),
			js.Null(),
		)
	}

	r.fbo_pp = make([]js.Value, 2)
	r.fbo_pp_texture = make([]js.Value, 2)

	// Shaders might use negative values, so
	// we specify that we want signed pixels
	// r.fbo_pp_texture
	for i := 0; i < 2; i++ {
		r.fbo_pp_texture[i] = webGL.Call("createTexture")
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), r.fbo_pp_texture[i])

		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("NEAREST"))
		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), webGL.Get("NEAREST"))
		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))

		// In WebGL we can't use RGBA8_SNORM, so we use RGBA
		webGL.Call("texImage2D",
			webGL.Get("TEXTURE_2D"),
			0,
			webGL.Get("RGBA"),
			sys.scrrect[2],
			sys.scrrect[3],
			0,
			webGL.Get("RGBA"),
			webGL.Get("UNSIGNED_BYTE"),
			nil,
		)
	}

	// done with r.fbo_texture, unbind it
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), nil)

	r.rbo_depth = webGL.Call("createRenderbuffer")

	webGL.Call("bindRenderbuffer", webGL.Get("RENDERBUFFER"), r.rbo_depth)

	if sys.msaa > 0 {
		webGL.Call("renderbufferStorageMultisample",
			webGL.Get("RENDERBUFFER"),
			sys.msaa,
			webGL.Get("DEPTH_COMPONENT16"),
			sys.scrrect[2],
			sys.scrrect[3],
		)

	} else {
		webGL.Call("renderbufferStorage",
			webGL.Get("RENDERBUFFER"),
			webGL.Get("DEPTH_COMPONENT16"),
			sys.scrrect[2],
			sys.scrrect[3],
		)

	}

	webGL.Call("bindRenderbuffer", webGL.Get("RENDERBUFFER"), nil)

	if sys.msaa > 0 {
		r.fbo_f_texture = r.newTexture(sys.scrrect[2], sys.scrrect[3], 32, false).(*TextureWeb)
		r.fbo_f_texture.SetData(nil)
	} else {
		//r.rbo_depth = webGL.Call("createRenderbuffer")
		//webGL.Call("bindRenderbuffer", webGL.Get("RENDERBUFFER"), r.rbo_depth)
		//webGL.Call("renderbufferStorage", webGL.Get("RENDERBUFFER"), webGL.Get("DEPTH_COMPONENT16"), sys.scrrect[2], sys.scrrect[3])
		//webGL.Call("bindRenderbuffer", webGL.Get("RENDERBUFFER"), nil)
	}

	// create an FBO for our r.fbo, which is then for r.fbo_texture
	r.fbo = webGL.Call("createFramebuffer")
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)

	if sys.msaa > 0 {
		webGL.Call("framebufferTexture2D", webGL.Get("FRAMEBUFFER"), webGL.Get("COLOR_ATTACHMENT0"), webGL.Get("TEXTURE_2D_MULTISAMPLE"), r.fbo_texture, 0)
		webGL.Call("framebufferRenderbuffer", webGL.Get("FRAMEBUFFER"), webGL.Get("DEPTH_ATTACHMENT"), webGL.Get("RENDERBUFFER"), r.rbo_depth)

		status := webGL.Call("checkFramebufferStatus", webGL.Get("FRAMEBUFFER"))
		if status.Int() != webGL.Get("FRAMEBUFFER_COMPLETE").Int() {
			sys.errLog.Printf("framebuffer create failed: 0x%x", status)
			fmt.Printf("framebuffer create failed: 0x%x \n", status)
		}

		r.fbo_f = webGL.Call("createFramebuffer")
		webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_f)
		webGL.Call("framebufferTexture2D", webGL.Get("FRAMEBUFFER"), webGL.Get("COLOR_ATTACHMENT0"), webGL.Get("TEXTURE_2D"), r.fbo_f_texture.handle, 0)
	} else {
		webGL.Call("framebufferTexture2D",
			webGL.Get("FRAMEBUFFER"),
			webGL.Get("COLOR_ATTACHMENT0"),
			webGL.Get("TEXTURE_2D"),
			r.fbo_texture,
			0,
		)
		webGL.Call("framebufferRenderbuffer",
			webGL.Get("FRAMEBUFFER"),
			webGL.Get("DEPTH_ATTACHMENT"),
			webGL.Get("RENDERBUFFER"),
			r.rbo_depth,
		)
	}

	// Check framebuffer status
	status := webGL.Call("checkFramebufferStatus", webGL.Get("FRAMEBUFFER"))
	if status.Int() != webGL.Get("FRAMEBUFFER_COMPLETE").Int() {
		sys.errLog.Printf("framebuffer create failed: 0x%x", status)
	}

	// create our two FBOs for our postprocessing needs
	r.fbo_pp = make([]js.Value, 2)
	for i := 0; i < 2; i++ {
		r.fbo_pp[i] = webGL.Call("createFramebuffer")
		webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_pp[i])
		webGL.Call("framebufferTexture2D",
			webGL.Get("FRAMEBUFFER"),
			webGL.Get("COLOR_ATTACHMENT0"),
			webGL.Get("TEXTURE_2D"),
			r.fbo_pp_texture[i],
			0,
		)
	}
	// create an FBO for our model stuff
	if r.enableModel {
		if r.enableShadow {
			// Create shadow framebuffer and cube textures
			r.fbo_shadow = webGL.Call("createFramebuffer")

			// Create and setup cube map textures for shadow mapping
			for i := 0; i < 4; i++ {
				r.fbo_shadow_cube_texture[i] = webGL.Call("createTexture")
				webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), r.fbo_shadow_cube_texture[i])

				for j := 0; j < 6; j++ {
					webGL.Call("texImage2D",
						webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X").Int()+j,
						0,
						webGL.Get("DEPTH_COMPONENT"),
						1024,
						1024,
						0,
						webGL.Get("DEPTH_COMPONENT"),
						webGL.Get("FLOAT"),
						nil,
					)
				}

				webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("NEAREST"))
				webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_MIN_FILTER"), webGL.Get("NEAREST"))
				webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
				webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))
			}

			// Setup shadow framebuffer
			webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_shadow)
			status := webGL.Call("checkFramebufferStatus", webGL.Get("FRAMEBUFFER"))
			if status.Int() != webGL.Get("FRAMEBUFFER_COMPLETE").Int() {
				sys.errLog.Printf("shadow framebuffer create failed: 0x%x", status)
			}
		}

		r.fbo_env = webGL.Call("createFramebuffer")
	}

	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), nil)
}

// ------------------------------------------------------------------
// ShaderProgram

type ShaderProgramWeb struct {
	// Program
	program js.Value
	// Attributes
	a map[string]js.Value
	// Uniforms
	u map[string]js.Value
	// TextureWeb units
	t map[string]int
}

func (r *RendererWeb) newShaderProgram(vert, frag, geo, id string, crashWhenFail bool) (s *ShaderProgramWeb, err error) {
	webGL := getWebGLContext()
	var vertObj, fragObj, geoObj, prog js.Value
	if vertObj, err = r.compileShader(webGL.Get("VERTEX_SHADER").String(), vert); chkEX(err, "Shader compilation error on "+id+"\n", crashWhenFail) {
		return nil, err
	}
	if fragObj, err = r.compileShader(webGL.Get("FRAGMENT_SHADER").String(), frag); chkEX(err, "Shader compilation error on "+id+"\n", crashWhenFail) {
		return nil, err
	}
	if len(geo) > 0 {

		if geoObj, err = r.compileShader(webGL.Get("GEOMETRY_SHADER").String(), geo); chkEX(err, "Shader compilation error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
		if prog, err = r.linkProgram(vertObj, fragObj, geoObj); chkEX(err, "Link program error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
	} else {
		if prog, err = r.linkProgram(vertObj, fragObj); chkEX(err, "Link program error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
	}
	s = &ShaderProgramWeb{program: prog}
	s.a = make(map[string]js.Value)
	s.u = make(map[string]js.Value)
	s.t = make(map[string]int)
	return s, nil
}

func (s *ShaderProgramWeb) RegisterAttributes(names ...string) {
	webGL := getWebGLContext()

	// Initialize attributes map if not exists
	if s.a == nil {
		s.a = make(map[string]js.Value)
	}

	for _, name := range names {
		// Get attribute location using WebGL
		location := webGL.Call("getAttribLocation", s.program, name)

		// Store location as int32 for compatibility with existing code
		s.a[name] = location
	}
}

func (s *ShaderProgramWeb) RegisterUniforms(names ...string) {
	webGL := getWebGLContext()

	// Initialize uniforms map if not exists
	if s.u == nil {
		s.u = make(map[string]js.Value)
	}

	for _, name := range names {
		// Get uniform location using WebGL
		location := webGL.Call("getUniformLocation", s.program, name)

		// Store location as int32 for compatibility with existing code
		if !location.IsNull() {
			s.u[name] = location
		} else {
			// Optional: Log warning for missing uniforms
			fmt.Printf("Warning: uniform '%s' not found in shader program\n", name)
		}
	}
}

func (s *ShaderProgramWeb) RegisterTextures(names ...string) {
	webGL := getWebGLContext()

	// Initialize textures map if not exists
	if s.u == nil {
		s.u = make(map[string]js.Value)
	}
	if s.t == nil {
		s.t = make(map[string]int)
	}

	for _, name := range names {
		// Get uniform location using WebGL
		location := webGL.Call("getUniformLocation", s.program, name)

		// Store uniform location as int32
		if !location.IsNull() {
			s.u[name] = location
		} else {
			// Optional: Log warning for missing uniforms
			fmt.Printf("Warning: texture uniform '%s' not found in shader program\n", name)
		}

		// Store texture unit index
		s.t[name] = len(s.t)
	}
}

func (r *RendererWeb) compileShader(shaderType string, src string) (shader js.Value, err error) {
	webGL := getWebGLContext()

	var shaderTypeGL js.Value
	switch shaderType {
	case webGL.Get("VERTEX_SHADER").String():
		shaderTypeGL = webGL.Get("VERTEX_SHADER")
	case webGL.Get("FRAGMENT_SHADER").String():
		shaderTypeGL = webGL.Get("FRAGMENT_SHADER")
	default:
		return js.Value{}, fmt.Errorf("unknown shader type: %v", shaderType)
	}

	shader = webGL.Call("createShader", shaderTypeGL)
	src = strings.Replace(src, "##version 300 es\n", "", 1)

	src = "##version 300 es\n" + src + "\x00"

	webGL.Call("shaderSource", shader, src)
	webGL.Call("compileShader", shader)

	success := webGL.Call("getShaderParameter", shader, webGL.Get("COMPILE_STATUS"))
	if !success.Bool() {
		info := webGL.Call("getShaderInfoLog", shader).String()
		webGL.Call("deleteShader", shader)
		return js.Value{}, fmt.Errorf("shader compilation failed: %v", info)
	}

	return shader, nil
}

func (r *RendererWeb) linkProgram(shaders ...js.Value) (program js.Value, err error) {
	webGL := getWebGLContext()

	program = webGL.Call("createProgram")
	for _, shader := range shaders {
		webGL.Call("attachShader", program, shader)
	}

	/*
		// WebGL doesn't support geometry shaders, but we can work around this
		// by disabling advanced shadow mapping and falling back to simpler techniques
		if len(shaders) > 2 {
			// Geometry Shader Params
			gl.ProgramParameteriARB(program, gl.GEOMETRY_INPUT_TYPE_ARB, gl.TRIANGLES)
			gl.ProgramParameteriARB(program, gl.GEOMETRY_OUTPUT_TYPE_ARB, gl.TRIANGLE_STRIP)
			gl.ProgramParameteriARB(program, gl.GEOMETRY_VERTICES_OUT_ARB, 3*6)
		}
	*/

	webGL.Call("linkProgram", program)
	success := webGL.Call("getProgramParameter", program, webGL.Get("LINK_STATUS"))
	if !success.Bool() {
		info := webGL.Call("getProgramInfoLog", program).String()

		// Delete program and shaders before returning error
		webGL.Call("deleteProgram", program)
		for _, shader := range shaders {
			webGL.Call("deleteShader", shader)
		}

		return js.Value{}, fmt.Errorf("program link failed: %v", info)
	}
	// Mark shaders for deletion when the program is deleted
	for _, shader := range shaders {
		webGL.Call("deleteShader", shader)
	}

	return program, nil
}

// ------------------------------------------------------------------
// TextureWeb

type TextureWeb struct {
	width  int32
	height int32
	depth  int32
	filter bool
	handle js.Value
}

// Generate a new texture name
func (r *RendererWeb) newTexture(width, height, depth int32, filter bool) (t Texture) {
	webGL := getWebGLContext()
	handle := webGL.Call("createTexture")

	t = &TextureWeb{
		width: width, height: height, depth: depth,
		filter: filter,
		handle: handle,
	}

	// Set active texture unit to 0
	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))

	// Bind the texture
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), handle)
	return t
}

func (r *RendererWeb) newDataTexture(width, height int32) (t Texture) {
	webGL := getWebGLContext()

	handle := webGL.Call("createTexture")

	t = &TextureWeb{
		width: width, height: height, depth: 32,
		filter: false, handle: handle,
	}

	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), handle)

	webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, webGL.Get("RGBA"), width, height, 0, webGL.Get("RGBA"), webGL.Get("FLOAT"), nil)

	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("NEAREST"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), webGL.Get("NEAREST"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))

	return t
}

func (r *RendererWeb) newHDRTexture(width, height int32) (t Texture) {
	webGL := getWebGLContext()

	handle := webGL.Call("createTexture")

	t = &TextureWeb{
		width: width, height: height, depth: 24,
		filter: false, handle: handle,
	}

	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), handle)

	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), webGL.Get("LINEAR"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("LINEAR"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("MIRRORED_REPEAT"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("MIRRORED_REPEAT"))
	return t
}

func (r *RendererWeb) newCubeMapTexture(widthHeight int32, mipmap bool) (t Texture) {
	webGL := getWebGLContext()

	handle := webGL.Call("createTexture")

	t = &TextureWeb{
		width: widthHeight, height: widthHeight, depth: 24,
		filter: false, handle: handle,
	}
	webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), handle)

	for i := 0; i < 6; i++ {
		webGL.Call("texImage2D",
			webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X").Int()+i,
			0,
			webGL.Get("RGB"),
			widthHeight,
			widthHeight,
			0,
			webGL.Get("RGB"),
			webGL.Get("FLOAT"),
			nil,
		)
	}
	if mipmap {
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_CUBE_MAP"),
			webGL.Get("TEXTURE_MIN_FILTER"),
			webGL.Get("LINEAR_MIPMAP_LINEAR"),
		)
		webGL.Call("generateMipmap", webGL.Get("TEXTURE_CUBE_MAP"))
	} else {
		webGL.Call("texParameteri",
			webGL.Get("TEXTURE_CUBE_MAP"),
			webGL.Get("TEXTURE_MIN_FILTER"),
			webGL.Get("LINEAR"),
		)
	}

	webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_MAG_FILTER"), webGL.Get("LINEAR"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_CUBE_MAP"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))

	return t
}

// Bind a texture and upload texel data to it
func (t *TextureWeb) SetData(data []byte) {
	webGL := getWebGLContext()

	var interp js.Value
	if t.filter {
		interp = webGL.Get("LINEAR")
	} else {
		interp = webGL.Get("NEAREST")
	}

	// Map internal format
	format := webGL.Get("RGBA")
	if t.depth == 8 {
		format = webGL.Get("LUMINANCE")
	} else if t.depth == 24 {
		format = webGL.Get("RGB")
	}

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("pixelStorei", webGL.Get("UNPACK_ALIGNMENT"), 1)

	if data != nil {
		// Create TypedArray from data
		jsData := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(jsData, data)

		webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, format, t.width, t.height, 0, format, webGL.Get("UNSIGNED_BYTE"), jsData)
	} else {
		webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, format, t.width, t.height, 0, format, webGL.Get("UNSIGNED_BYTE"), nil)
	}

	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), interp)
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), interp)
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), webGL.Get("CLAMP_TO_EDGE"))
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), webGL.Get("CLAMP_TO_EDGE"))
}
func (t *TextureWeb) SetDataG(data []byte, mag, min, ws, wt int32) {
	webGL := getWebGLContext()

	format := webGL.Get("RGBA")
	if t.depth == 8 {
		format = webGL.Get("LUMINANCE")
	} else if t.depth == 24 {
		format = webGL.Get("RGB")
	}

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("pixelStorei", webGL.Get("UNPACK_ALIGNMENT"), 1)

	jsData := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsData, data)

	// Upload texture data
	webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, format, t.width, t.height, 0, format, webGL.Get("UNSIGNED_BYTE"), jsData)

	webGL.Call("generateMipmap", webGL.Get("TEXTURE_2D"))

	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), mag)
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), min)
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_S"), ws)
	webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_WRAP_T"), wt)
}
func (t *TextureWeb) SetPixelData(data []float32) {
	webGL := getWebGLContext()

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("pixelStorei", webGL.Get("UNPACK_ALIGNMENT"), 1)

	// Create TypedArray from float32 data
	jsData := js.Global().Get("Float32Array").New(len(data))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*4))

	// Upload texture data using RGBA32F format
	webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, webGL.Get("RGBA"), t.width, t.height, 0, webGL.Get("RGBA"), webGL.Get("FLOAT"), jsData)
}
func (t *TextureWeb) SetRGBPixelData(data []float32) {
	webGL := getWebGLContext()

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("pixelStorei", webGL.Get("UNPACK_ALIGNMENT"), 1)

	// Create TypedArray from float32 data
	jsData := js.Global().Get("Float32Array").New(len(data))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*4))

	// Upload texture data using RGB format
	webGL.Call("texImage2D", webGL.Get("TEXTURE_2D"), 0, webGL.Get("RGB"), t.width, t.height, 0, webGL.Get("RGB"), webGL.Get("FLOAT"), jsData)
}

// Return whether texture has a valid handle
func (t *TextureWeb) IsValid() bool {
	return t.width != 0 && t.height != 0 && !t.handle.IsNull()
}

func (t *TextureWeb) GetWidth() int32 {
	return t.width
}

func (t *TextureWeb) MapInternalFormat(i int32) js.Value {
	webGL := getWebGLContext()

	// Map bit depths to WebGL internal formats
	switch i {
	case 8:
		return webGL.Get("LUMINANCE")
	case 24:
		return webGL.Get("RGB")
	case 32:
		return webGL.Get("RGBA")
	default:
		return webGL.Get("RGBA") // Default to RGBA
	}
}

func (r *RendererWeb) GetName() string {
	return "WebGL"
}

// init 3D model shader
func (r *RendererWeb) InitModelShader() error {
	var err error
	if r.enableShadow {
		r.modelShader, err = r.newShaderProgram(modelVertShader, "#define ENABLE_SHADOW\n"+modelFragShader, "", "Model Shader", false)
	} else {
		r.modelShader, err = r.newShaderProgram(modelVertShader, modelFragShader, "", "Model Shader", false)
	}
	if err != nil {
		return err
	}
	r.modelShader.RegisterAttributes("vertexId", "position", "uv", "normalIn", "tangentIn", "vertColor", "joints_0", "joints_1", "weights_0", "weights_1")
	r.modelShader.RegisterUniforms("model", "view", "projection", "farPlane", "normalMatrix", "unlit", "baseColorFactor", "add", "mult", "useTexture", "useNormalMap", "useMetallicRoughnessMap", "useEmissionMap", "neg", "gray", "hue",
		"enableAlpha", "alphaThreshold", "numJoints", "morphTargetWeight", "morphTargetOffset", "morphTargetTextureDimension", "numTargets", "numVertices",
		"metallicRoughness", "ambientOcclusionStrength", "emission", "environmentIntensity", "mipCount",
		"cameraPosition", "environmentRotation", "texTransform", "normalMapTransform", "metallicRoughnessMapTransform", "ambientOcclusionMapTransform", "emissionMapTransform",
		"lightMatrices[0]", "lightMatrices[1]", "lightMatrices[2]", "lightMatrices[3]",
		"lights[0].direction", "lights[0].range", "lights[0].color", "lights[0].intensity", "lights[0].position", "lights[0].innerConeCos", "lights[0].outerConeCos", "lights[0].type", "lights[0].shadowBias", "lights[0].shadowMapFar",
		"lights[1].direction", "lights[1].range", "lights[1].color", "lights[1].intensity", "lights[1].position", "lights[1].innerConeCos", "lights[1].outerConeCos", "lights[1].type", "lights[1].shadowBias", "lights[1].shadowMapFar",
		"lights[2].direction", "lights[2].range", "lights[2].color", "lights[2].intensity", "lights[2].position", "lights[2].innerConeCos", "lights[2].outerConeCos", "lights[2].type", "lights[2].shadowBias", "lights[2].shadowMapFar",
		"lights[3].direction", "lights[3].range", "lights[3].color", "lights[3].intensity", "lights[3].position", "lights[3].innerConeCos", "lights[3].outerConeCos", "lights[3].type", "lights[3].shadowBias", "lights[3].shadowMapFar",
	)
	r.modelShader.RegisterTextures("tex", "morphTargetValues", "jointMatrices", "normalMap", "metallicRoughnessMap", "ambientOcclusionMap", "emissionMap", "lambertianEnvSampler", "GGXEnvSampler", "GGXLUT",
		"shadowCubeMap[0]", "shadowCubeMap[1]", "shadowCubeMap[2]", "shadowCubeMap[3]")

	if r.enableShadow {
		r.shadowMapShader, err = r.newShaderProgram(shadowVertShader, shadowFragShader, shadowGeoShader, "Shadow Map Shader", false)
		if err != nil {
			return err
		}
		r.shadowMapShader.RegisterAttributes("vertexId", "position", "vertColor", "uv", "joints_0", "joints_1", "weights_0", "weights_1")
		r.shadowMapShader.RegisterUniforms("model", "lightMatrices[0]", "lightMatrices[1]", "lightMatrices[2]", "lightMatrices[3]", "lightMatrices[4]", "lightMatrices[5]",
			"lightMatrices[6]", "lightMatrices[7]", "lightMatrices[8]", "lightMatrices[9]", "lightMatrices[10]", "lightMatrices[11]",
			"lightMatrices[12]", "lightMatrices[13]", "lightMatrices[14]", "lightMatrices[15]", "lightMatrices[16]", "lightMatrices[17]",
			"lightMatrices[18]", "lightMatrices[19]", "lightMatrices[20]", "lightMatrices[21]", "lightMatrices[22]", "lightMatrices[23]",
			"lightType[0]", "lightType[1]", "lightType[2]", "lightType[3]", "lightPos[0]", "lightPos[1]", "lightPos[2]", "lightPos[3]",
			"farPlane", "numJoints", "morphTargetWeight", "morphTargetOffset", "morphTargetTextureDimension", "numTargets", "numVertices", "enableAlpha", "alphaThreshold", "baseColorFactor", "useTexture", "texTransform", "lightIndex")
		r.shadowMapShader.RegisterTextures("morphTargetValues", "jointMatrices", "tex")
	}
	r.panoramaToCubeMapShader, err = r.newShaderProgram(identVertShader, panoramaToCubeMapFragShader, "", "Panorama To Cubemap Shader", false)
	if err != nil {
		return err
	}
	r.panoramaToCubeMapShader.RegisterAttributes("VertCoord")
	r.panoramaToCubeMapShader.RegisterUniforms("currentFace")
	r.panoramaToCubeMapShader.RegisterTextures("panorama")

	r.cubemapFilteringShader, err = r.newShaderProgram(identVertShader, cubemapFilteringFragShader, "", "Cubemap Filtering Shader", false)
	if err != nil {
		return err
	}
	r.cubemapFilteringShader.RegisterAttributes("VertCoord")
	r.cubemapFilteringShader.RegisterUniforms("sampleCount", "distribution", "width", "currentFace", "roughness", "intensityScale", "isLUT")
	r.cubemapFilteringShader.RegisterTextures("cubeMap")
	return nil
}

func (r *RendererWeb) Close() {
}

func (r *RendererWeb) IsModelEnabled() bool {
	return r.enableModel
}

func (r *RendererWeb) IsShadowEnabled() bool {
	return r.enableShadow
}
func (r *RendererWeb) BeginFrame(clearColor bool) {
	sys.absTickCountF++
	webGL := getWebGLContext()
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)

	webGL.Call("viewport", 0, 0, sys.scrrect[2], sys.scrrect[3])

	if clearColor {
		webGL.Call("clear",
			webGL.Get("COLOR_BUFFER_BIT").Int()|webGL.Get("DEPTH_BUFFER_BIT").Int(),
		)
	} else {
		webGL.Call("clear", webGL.Get("DEPTH_BUFFER_BIT"))
	}
}

func (r *RendererWeb) BlendReset() {
	webGL := getWebGLContext()

	webGL.Call("blendEquation", webGL.Get("FUNC_ADD"))

	// Set blend function to standard alpha blending
	webGL.Call("blendFunc",
		webGL.Get("SRC_ALPHA"), webGL.Get("ONE_MINUS_SRC_ALPHA"),
	)
}
func (r *RendererWeb) EndFrame() {
	x, y, width, height := int32(0), int32(0), int32(sys.scrrect[2]), int32(sys.scrrect[3])
	time := getJSTimestamp() // consistent time across all shaders
	webGL := getWebGLContext()

	if sys.msaa > 0 {
		webGL.Call("bindFramebuffer", webGL.Get("DRAW_FRAMEBUFFER"), r.fbo_f)
		webGL.Call("bindFramebuffer", webGL.Get("READ_FRAMEBUFFER"), r.fbo)
		webGL.Call("blitFramebuffer", x, y, width, height, x, y, width, height, webGL.Get("COLOR_BUFFER_BIT"), webGL.Get("LINEAR"))
	}

	var scaleMode js.Value
	if sys.cfg.Video.WindowScaleMode {
		scaleMode = webGL.Get("LINEAR")
	} else {
		scaleMode = webGL.Get("NEAREST")
	}

	// set the viewport to the unscaled bounds for post-processing
	webGL.Call("viewport", x, y, width, height)
	// clear both of our post-processing FBOs to make sure
	// nothing's there. the output is set later
	for i := 0; i < 2; i++ {
		webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_pp[i])
		webGL.Call("clear", webGL.Get("COLOR_BUFFER_BIT"))
	}
	webGL.Call("activeTexture", webGL.Get("TEXTURE0"))

	fbo_texture := r.fbo_texture

	if sys.msaa > 0 {
		fbo_texture = r.fbo_f_texture.handle
	}

	// disable blending
	webGL.Call("disable", webGL.Get("BLEND"))

	for i := 0; i < len(r.postShaderSelect); i++ {
		postShader := r.postShaderSelect[i]

		// this is here because it is undefined
		// behavior to write to the same FBO
		if i%2 == 0 {
			// ping! our first post-processing FBO is the output
			webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_pp[0])

			if i == 0 {
				// first pass, use fbo_texture
				webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), fbo_texture)
			} else {
				// not the first pass, use the second post-processing FBO
				webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), r.fbo_pp_texture[1])
			}
		} else {
			// pong! our second post-processing FBO is the output
			webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_pp[1])
			// our first post-processing FBO is the input
			webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), r.fbo_pp_texture[0])
		}

		if i >= len(r.postShaderSelect)-1 {
			// this is the last shader,
			// so we ask GL to scale it and output it
			// to FB0, the default frame buffer that the user sees
			x, y, width, height := sys.window.GetScaledViewportSize()
			webGL.Call("viewport", x, y, width, height)
			webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), nil)
			// clear FB0 just to make sure
			webGL.Call("clear",
				webGL.Get("COLOR_BUFFER_BIT").Int()|webGL.Get("DEPTH_BUFFER_BIT").Int(),
			)
		}

		// tell GL we want to use our shader program
		webGL.Call("useProgram", postShader.program)

		// set post-processing parameters
		webGL.Call("uniform1i", postShader.u["Texture_GL21"], 0)
		webGL.Call("uniform2f", postShader.u["TextureSize"], float32(width), float32(height))
		webGL.Call("uniform1f", postShader.u["CurrentTime"], float32(time))
		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MAG_FILTER"), scaleMode)
		webGL.Call("texParameteri", webGL.Get("TEXTURE_2D"), webGL.Get("TEXTURE_MIN_FILTER"), scaleMode)

		// this actually draws the image to the FBO
		// by constructing a quad (2 tris)
		webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.postVertBuffer)

		// construct the UVs of the quad
		loc := postShader.a["VertCoord"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, 0)

		// construct the quad and draw it
		webGL.Call("drawArrays", webGL.Get("TRIANGLE_STRIP"), 0, 4)
		webGL.Call("disableVertexAttribArray", loc)
	}
}

func (r *RendererWeb) Await() {
	webGL := getWebGLContext()
	webGL.Call("finish")
}

func (r *RendererWeb) MapBlendEquation(i BlendEquation) js.Value {
	webGL := getWebGLContext()

	var BlendEquationLUT = map[BlendEquation]js.Value{
		BlendAdd:             webGL.Get("FUNC_ADD"),
		BlendReverseSubtract: webGL.Get("FUNC_REVERSE_SUBTRACT"),
	}

	if value, ok := BlendEquationLUT[i]; ok {
		return value
	}
	// Default to FUNC_ADD if equation not found
	return webGL.Get("FUNC_ADD")
}

func (r *RendererWeb) MapBlendFunction(i BlendFunc) js.Value {
	webGL := getWebGLContext()

	var BlendFunctionLUT = map[BlendFunc]js.Value{
		BlendOne:              webGL.Get("ONE"),
		BlendZero:             webGL.Get("ZERO"),
		BlendSrcAlpha:         webGL.Get("SRC_ALPHA"),
		BlendOneMinusSrcAlpha: webGL.Get("ONE_MINUS_SRC_ALPHA"),
		BlendOneMinusDstColor: webGL.Get("ONE_MINUS_DST_COLOR"),
		BlendDstColor:         webGL.Get("DST_COLOR"),
	}
	return BlendFunctionLUT[i]
}

func (r *RendererWeb) MapPrimitiveMode(i PrimitiveMode) js.Value {
	webGL := getWebGLContext()
	var PrimitiveModeLUT = map[PrimitiveMode]js.Value{
		LINES:          webGL.Get("LINES"),
		LINE_LOOP:      webGL.Get("LINE_LOOP"),
		LINE_STRIP:     webGL.Get("LINE_STRIP"),
		TRIANGLES:      webGL.Get("TRIANGLES"),
		TRIANGLE_STRIP: webGL.Get("TRIANGLE_STRIP"),
		TRIANGLE_FAN:   webGL.Get("TRIANGLE_FAN"),
	}
	return PrimitiveModeLUT[i]
}

func (r *RendererWeb) SetPipeline(eq BlendEquation, src, dst BlendFunc) {
	webGL := getWebGLContext()

	webGL.Call("useProgram", r.spriteShader.program)

	webGL.Call("blendEquation", r.MapBlendEquation(eq))
	webGL.Call("blendFunc", r.MapBlendFunction(src), r.MapBlendFunction(dst))
	webGL.Call("enable", webGL.Get("BLEND"))

	// Must bind buffer before enabling attributes
	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.vertexBuffer)

	// Set position attribute
	loc := r.spriteShader.a["position"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 16, 0)

	// Set UV attribute
	loc = r.spriteShader.a["uv"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 16, 8)
}

func (r *RendererWeb) ReleasePipeline() {
	webGL := getWebGLContext()

	loc := r.spriteShader.a["position"]
	webGL.Call("disableVertexAttribArray", loc)

	loc = r.spriteShader.a["uv"]
	webGL.Call("disableVertexAttribArray", loc)

	webGL.Call("disable", webGL.Get("BLEND"))
}

func (r *RendererWeb) prepareShadowMapPipeline() {
	webGL := getWebGLContext()

	webGL.Call("useProgram", r.shadowMapShader.program)
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_shadow)
	webGL.Call("viewport", 0, 0, 1024, 1024)

	// Apparently we don't need to enable Texture_2d
	// webGL.Call("enable", webGL.Get("TEXTURE_2D"))
	webGL.Call("disable", webGL.Get("BLEND"))
	webGL.Call("enable", webGL.Get("DEPTH_TEST"))

	// Set depth function and mask
	//webGL.Call("depthFunc", webGL.Get("LESS"))
	//webGL.Call("depthMask", true)

	webGL.Call("blendEquation", webGL.Get("FUNC_ADD"))
	webGL.Call("blendFunc", webGL.Get("ONE"), webGL.Get("ZERO"))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.stageVertexBuffer)
	webGL.Call("bindBuffer", webGL.Get("ELEMENT_ARRAY_BUFFER"), r.stageIndexBuffer)

}
func (r *RendererWeb) setShadowMapPipeline(doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
	webGL := getWebGLContext()

	if invertFrontFace {
		webGL.Call("frontFace", webGL.Get("CW"))
	} else {
		webGL.Call("frontFace", webGL.Get("CCW"))
	}
	if !doubleSided {
		webGL.Call("enable", webGL.Get("CULL_FACE"))
		webGL.Call("cullFace", webGL.Get("BACK"))
	} else {
		webGL.Call("disable", webGL.Get("CULL_FACE"))
	}

	loc := r.shadowMapShader.a["vertexId"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 1, webGL.Get("INT"), false, 0, vertAttrOffset)
	offset := vertAttrOffset + 4*numVertices

	loc = r.shadowMapShader.a["position"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 3, webGL.Get("FLOAT"), false, 0, offset)
	offset += 12 * numVertices

	if useUV {
		loc = r.shadowMapShader.a["uv"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, offset)
		offset += 8 * numVertices
	} else {
		loc = r.shadowMapShader.a["uv"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib2f", loc, 0, 0)
	}

	if useNormal {
		offset += 12 * numVertices
	}
	if useTangent {
		offset += 16 * numVertices
	}
	if useVertColor {
		loc = r.shadowMapShader.a["vertColor"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices
	} else {
		loc = r.shadowMapShader.a["vertColor"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 1, 1, 1, 1)
	}
	if useJoint0 {
		loc = r.shadowMapShader.a["joints_0"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices

		loc = r.shadowMapShader.a["weights_0"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices

		if useJoint1 {
			loc = r.shadowMapShader.a["joints_1"]
			webGL.Call("enableVertexAttribArray", loc)
			webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
			offset += 16 * numVertices

			loc = r.shadowMapShader.a["weights_1"]
			webGL.Call("enableVertexAttribArray", loc)
			webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
			offset += 16 * numVertices
		} else {
			loc = r.shadowMapShader.a["joints_1"]
			webGL.Call("disableVertexAttribArray", loc)
			webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

			loc = r.shadowMapShader.a["weights_1"]
			webGL.Call("disableVertexAttribArray", loc)
			webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)
		}
	} else {
		loc = r.shadowMapShader.a["joints_0"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.shadowMapShader.a["weights_0"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.shadowMapShader.a["joints_1"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.shadowMapShader.a["weights_1"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)
	}
}

func (r *RendererWeb) ReleaseShadowPipeline() {
	webGL := getWebGLContext()

	loc := r.modelShader.a["vertexId"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["position"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["uv"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["vertColor"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["joints_0"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["weights_0"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["joints_1"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["weights_1"]
	webGL.Call("disableVertexAttribArray", loc)

	//webGL.Call("disable", webGL.Get("TEXTURE_2D"))
	webGL.Call("depthMask", true)
	webGL.Call("disable", webGL.Get("DEPTH_TEST"))
	webGL.Call("disable", webGL.Get("CULL_FACE"))
	webGL.Call("disable", webGL.Get("BLEND"))
}
func (r *RendererWeb) prepareModelPipeline(env *Environment) {
	webGL := getWebGLContext()

	webGL.Call("useProgram", r.modelShader.program)
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)
	webGL.Call("viewport", 0, 0, sys.scrrect[2], sys.scrrect[3])
	webGL.Call("clear", webGL.Get("DEPTH_BUFFER_BIT"))

	webGL.Call("enable", webGL.Get("BLEND"))
	// We don't need to enable these
	//webGL.Call("enable", webGL.Get("TEXTURE_2D"))
	//webGL.Call("enable", webGL.Get("TEXTURE_CUBE_MAP"))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.stageVertexBuffer)
	webGL.Call("bindBuffer", webGL.Get("ELEMENT_ARRAY_BUFFER"), r.stageIndexBuffer)
	if r.enableShadow {
		for i := 0; i < 4; i++ {
			loc := r.modelShader.u["shadowCubeMap["+strconv.Itoa(i)+"]"]
			unit := r.modelShader.t["shadowCubeMap["+strconv.Itoa(i)+"]"]

			webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
			webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), r.fbo_shadow_cube_texture[i])
			webGL.Call("uniform1i", loc, unit)
		}
	}
	if env != nil {
		loc := r.modelShader.u["lambertianEnvSampler"]
		unit := r.modelShader.t["lambertianEnvSampler"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), env.lambertianTexture.tex.(*TextureWeb).handle)
		webGL.Call("uniform1i", loc, unit)

		loc = r.modelShader.u["GGXEnvSampler"]
		unit = r.modelShader.t["GGXEnvSampler"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), env.GGXTexture.tex.(*TextureWeb).handle)
		webGL.Call("uniform1i", loc, unit)

		loc = r.modelShader.u["GGXLUT"]
		unit = r.modelShader.t["GGXLUT"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), env.GGXLUT.tex.(*TextureWeb).handle)
		webGL.Call("uniform1i", loc, unit)

		webGL.Call("uniform1f", r.modelShader.u["environmentIntensity"], env.environmentIntensity)
		webGL.Call("uniform1i", r.modelShader.u["mipCount"], env.mipmapLevels)

		rotationMatrix := mgl.Rotate3DX(math.Pi).Mul3(mgl.Rotate3DY(0.5 * math.Pi))
		rotationM := rotationMatrix[:]
		webGL.Call("uniformMatrix3fv", r.modelShader.u["environmentRotation"], false, rotationM)

	} else {

		unit := r.modelShader.t["lambertianEnvSampler"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), nil)
		webGL.Call("uniform1i", r.modelShader.u["lambertianEnvSampler"], unit)

		unit = r.modelShader.t["GGXEnvSampler"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), nil)
		webGL.Call("uniform1i", r.modelShader.u["GGXEnvSampler"], unit)

		unit = r.modelShader.t["GGXLUT"]
		webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
		webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), nil)
		webGL.Call("uniform1i", r.modelShader.u["GGXLUT"], unit)

		webGL.Call("uniform1f", r.modelShader.u["environmentIntensity"], 0)
	}
}
func (r *RendererWeb) SetModelPipeline(eq BlendEquation, src, dst BlendFunc, depthTest, depthMask, doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
	webGL := getWebGLContext()

	if depthTest {
		webGL.Call("enable", webGL.Get("DEPTH_TEST"))
		webGL.Call("depthFunc", webGL.Get("LESS"))
	} else {
		webGL.Call("disable", webGL.Get("DEPTH_TEST"))
	}
	webGL.Call("depthMask", depthMask)

	if invertFrontFace {
		webGL.Call("frontFace", webGL.Get("CW"))
	} else {
		webGL.Call("frontFace", webGL.Get("CCW"))
	}
	if !doubleSided {
		webGL.Call("enable", webGL.Get("CULL_FACE"))
		webGL.Call("cullFace", webGL.Get("BACK"))
	} else {
		webGL.Call("disable", webGL.Get("CULL_FACE"))
	}

	webGL.Call("blendEquation", r.MapBlendEquation(eq))
	webGL.Call("blendFunc", r.MapBlendFunction(src), r.MapBlendFunction(dst))

	loc := r.modelShader.a["vertexId"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 1, webGL.Get("INT"), false, 0, vertAttrOffset)
	offset := vertAttrOffset + 4*numVertices

	loc = r.modelShader.a["position"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 3, webGL.Get("FLOAT"), false, 0, offset)
	offset += 12 * numVertices

	if useUV {
		loc = r.modelShader.a["uv"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, offset)
		offset += 8 * numVertices
	} else {
		loc = r.modelShader.a["uv"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib2f", loc, 0, 0)
	}
	if useNormal {
		loc = r.modelShader.a["normalIn"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 3, webGL.Get("FLOAT"), false, 0, offset)
		offset += 12 * numVertices
	} else {
		loc = r.modelShader.a["normalIn"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib3f", loc, 0, 0, 0)
	}
	if useTangent {
		loc = r.modelShader.a["tangentIn"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices
	} else {
		loc = r.modelShader.a["tangentIn"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)
	}
	if useVertColor {
		loc = r.modelShader.a["vertColor"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices
	} else {
		loc = r.modelShader.a["vertColor"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 1, 1, 1, 1)
	}
	if useJoint0 {
		loc = r.modelShader.a["joints_0"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices

		loc = r.modelShader.a["weights_0"]
		webGL.Call("enableVertexAttribArray", loc)
		webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
		offset += 16 * numVertices

		if useJoint1 {
			loc = r.modelShader.a["joints_1"]
			webGL.Call("enableVertexAttribArray", loc)
			webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
			offset += 16 * numVertices

			loc = r.modelShader.a["weights_1"]
			webGL.Call("enableVertexAttribArray", loc)
			webGL.Call("vertexAttribPointer", loc, 4, webGL.Get("FLOAT"), false, 0, offset)
			offset += 16 * numVertices
		} else {
			loc = r.modelShader.a["joints_1"]
			webGL.Call("disableVertexAttribArray", loc)
			webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

			loc = r.modelShader.a["weights_1"]
			webGL.Call("disableVertexAttribArray", loc)
			webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)
		}
	} else {
		loc = r.modelShader.a["joints_0"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.modelShader.a["weights_0"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.modelShader.a["joints_1"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)

		loc = r.modelShader.a["weights_1"]
		webGL.Call("disableVertexAttribArray", loc)
		webGL.Call("vertexAttrib4f", loc, 0, 0, 0, 0)
	}
}
func (r *RendererWeb) ReleaseModelPipeline() {
	webGL := getWebGLContext()

	loc := r.modelShader.a["vertexId"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["position"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["uv"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["vertColor"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["joints_0"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["weights_0"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["joints_1"]
	webGL.Call("disableVertexAttribArray", loc)
	loc = r.modelShader.a["weights_1"]
	webGL.Call("disableVertexAttribArray", loc)
	//webGL.Call("disable", webGL.Get("TEXTURE_2D"))
	webGL.Call("depthMask", true)
	webGL.Call("disable", webGL.Get("DEPTH_TEST"))
	webGL.Call("disable", webGL.Get("CULL_FACE"))
	webGL.Call("disable", webGL.Get("BLEND"))
}

func (r *RendererWeb) ProcessShadowMapTexture(index int) {
	webGL := getWebGLContext()

	webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), r.fbo_shadow_cube_texture[index])

	data := js.Global().Get("Float32Array").New(1024 * 1024)

	// Get depth texture data from positive X face
	webGL.Call("getTexImage",
		webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X"),
		0,
		webGL.Get("DEPTH_COMPONENT"),
		webGL.Get("FLOAT"),
		data,
	)

	for i := 0; i < 4; i++ {
		webGL.Call("texImage2D",
			webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X").Int()+i+2,
			0,
			webGL.Get("DEPTH_COMPONENT"),
			1024,
			1024,
			0,
			webGL.Get("DEPTH_COMPONENT"),
			webGL.Get("FLOAT"),
			data,
		)
	}
}

func (r *RendererWeb) ReadPixels(data []uint8, width, height int) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Uint8Array").New(width * height * 4)

	webGL.Call("readPixels",
		0, 0,
		width, height,
		webGL.Get("RGBA"),
		webGL.Get("UNSIGNED_BYTE"),
		jsData,
	)

	// Copy data back to Go slice
	js.CopyBytesToGo(data, jsData)
}

func (r *RendererWeb) Scissor(x, y, width, height int32) {
	webGL := getWebGLContext()
	webGL.Call("enable", webGL.Get("SCISSOR_TEST"))
	webGL.Call("scissor",
		x, sys.scrrect[3]-(y+height),
		width, height,
	)
}

func (r *RendererWeb) DisableScissor() {
	webGL := getWebGLContext()
	webGL.Call("disable", webGL.Get("SCISSOR_TEST"))
}

func (r *RendererWeb) SetUniformI(name string, val int) {
	webGL := getWebGLContext()
	loc := r.spriteShader.u[name]
	webGL.Call("uniform1i", loc, val)
}

func (r *RendererWeb) SetUniformF(name string, values ...float32) {
	webGL := getWebGLContext()
	loc := r.spriteShader.u[name]
	switch len(values) {
	case 1:
		webGL.Call("uniform1f", loc, values[0])
	case 2:
		webGL.Call("uniform2f", loc, values[0], values[1])
	case 3:
		webGL.Call("uniform3f", loc, values[0], values[1], values[2])
	case 4:
		webGL.Call("uniform4f", loc, values[0], values[1], values[2], values[3])
	}
}

func (r *RendererWeb) SetUniformFv(name string, values []float32) {
	webGL := getWebGLContext()
	loc := r.spriteShader.u[name]

	jsData := js.Global().Get("Float32Array").New(len(values))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))

	switch len(values) {
	case 2:
		webGL.Call("uniform2fv", loc, jsData)
	case 3:
		webGL.Call("uniform3fv", loc, jsData)
	case 4:
		webGL.Call("uniform4fv", loc, jsData)
	}
}

func (r *RendererWeb) SetUniformMatrix(name string, value []float32) {
	webGL := getWebGLContext()
	loc := r.spriteShader.u[name]

	// Create Float32Array from the matrix data
	jsData := js.Global().Get("Float32Array").New(len(value))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&value[0])), len(value)*4))

	// Set uniform matrix value
	webGL.Call("uniformMatrix4fv", loc, false, jsData)
}

func (r *RendererWeb) SetTexture(name string, tex Texture) {
	webGL := getWebGLContext()
	t := tex.(*TextureWeb)
	loc, unit := r.spriteShader.u[name], r.spriteShader.t[name]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("uniform1i", loc, unit)
}

func (r *RendererWeb) SetModelUniformI(name string, val int) {
	webGL := getWebGLContext()
	loc := r.modelShader.u[name]
	webGL.Call("uniform1i", loc, val)
}

func (r *RendererWeb) SetModelUniformF(name string, values ...float32) {
	webGL := getWebGLContext()
	loc := r.modelShader.u[name]
	switch len(values) {
	case 1:
		webGL.Call("uniform1f", loc, values[0])
	case 2:
		webGL.Call("uniform2f", loc, values[0], values[1])
	case 3:
		webGL.Call("uniform3f", loc, values[0], values[1], values[2])
	case 4:
		webGL.Call("uniform4f", loc, values[0], values[1], values[2], values[3])
	}
}
func (r *RendererWeb) SetModelUniformFv(name string, values []float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(values))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))

	loc := r.modelShader.u[name]
	switch len(values) {
	case 2:
		webGL.Call("uniform2fv", loc, jsData)
	case 3:
		webGL.Call("uniform3fv", loc, jsData)
	case 4:
		webGL.Call("uniform4fv", loc, jsData)
	case 8:
		webGL.Call("uniform4fv", loc, jsData)
	}
}
func (r *RendererWeb) SetModelUniformMatrix(name string, value []float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(value))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&value[0])), len(value)*4))

	loc := r.modelShader.u[name]
	webGL.Call("uniformMatrix4fv", loc, false, jsData)
}

func (r *RendererWeb) SetModelUniformMatrix3(name string, value []float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(value))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&value[0])), len(value)*4))

	loc := r.modelShader.u[name]
	webGL.Call("uniformMatrix3fv", loc, false, jsData)
}

func (r *RendererWeb) SetModelTexture(name string, tex Texture) {
	webGL := getWebGLContext()

	t := tex.(*TextureWeb)
	loc, unit := r.modelShader.u[name], r.modelShader.t[name]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("uniform1i", loc, unit)
}

func (r *RendererWeb) SetShadowMapUniformI(name string, val int) {
	webGL := getWebGLContext()

	loc := r.shadowMapShader.u[name]
	webGL.Call("uniform1i", loc, val)
}

func (r *RendererWeb) SetShadowMapUniformF(name string, values ...float32) {
	webGL := getWebGLContext()

	loc := r.shadowMapShader.u[name]
	switch len(values) {
	case 1:
		webGL.Call("uniform1f", loc, values[0])
	case 2:
		webGL.Call("uniform2f", loc, values[0], values[1])
	case 3:
		webGL.Call("uniform3f", loc, values[0], values[1], values[2])
	case 4:
		webGL.Call("uniform4f", loc, values[0], values[1], values[2], values[3])
	}
}
func (r *RendererWeb) SetShadowMapUniformFv(name string, values []float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(values))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))

	loc := r.shadowMapShader.u[name]
	switch len(values) {
	case 2:
		webGL.Call("uniform2fv", loc, jsData)
	case 3:
		webGL.Call("uniform3fv", loc, jsData)
	case 4:
		webGL.Call("uniform4fv", loc, jsData)
	case 8:
		webGL.Call("uniform4fv", loc, jsData) // For two vec4s
	}
}
func (r *RendererWeb) SetShadowMapUniformMatrix(name string, value []float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(value))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&value[0])), len(value)*4))

	loc := r.shadowMapShader.u[name]
	webGL.Call("uniformMatrix4fv", loc, false, jsData)
}

func (r *RendererWeb) SetShadowMapTexture(name string, tex Texture) {
	webGL := getWebGLContext()

	t := tex.(*TextureWeb)
	loc, unit := r.shadowMapShader.u[name], r.shadowMapShader.t[name]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), t.handle)
	webGL.Call("uniform1i", loc, unit)
}

func (r *RendererWeb) SetShadowFrameTexture(i uint32) {
	webGL := getWebGLContext()
	//gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, r.fbo_shadow_cube_texture[i], 0)
	webGL.Call("framebufferTextureLayer",
		webGL.Get("FRAMEBUFFER"),
		webGL.Get("DEPTH_ATTACHMENT"),
		r.fbo_shadow_cube_texture[i],
		0, // mipmap level
		0, // layer
	)
	webGL.Call("clear", webGL.Get("DEPTH_BUFFER_BIT"))
}

func (r *RendererWeb) SetShadowFrameCubeTexture(i uint32) {
	webGL := getWebGLContext()
	webGL.Call("framebufferTexture",
		webGL.Get("FRAMEBUFFER"),
		webGL.Get("DEPTH_ATTACHMENT"),
		r.fbo_shadow_cube_texture[i],
		0,
	)
	webGL.Call("clear", webGL.Get("DEPTH_BUFFER_BIT"))
}

func (r *RendererWeb) SetVertexData(values ...float32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Float32Array").New(len(values))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.vertexBuffer)
	webGL.Call("bufferData",
		webGL.Get("ARRAY_BUFFER"),
		jsData,
		webGL.Get("STATIC_DRAW"),
	)
}
func (r *RendererWeb) SetStageVertexData(values []byte) {
	webGL := getWebGLContext()
	jsData := js.Global().Get("Uint8Array").New(len(values))
	js.CopyBytesToJS(jsData, values)

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.stageVertexBuffer)
	webGL.Call("bufferData",
		webGL.Get("ARRAY_BUFFER"),
		jsData,
		webGL.Get("STATIC_DRAW"),
	)
}
func (r *RendererWeb) SetStageIndexData(values ...uint32) {
	webGL := getWebGLContext()

	jsData := js.Global().Get("Uint32Array").New(len(values))
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))

	webGL.Call("bindBuffer", webGL.Get("ELEMENT_ARRAY_BUFFER"), r.stageIndexBuffer)
	webGL.Call("bufferData",
		webGL.Get("ELEMENT_ARRAY_BUFFER"),
		jsData,
		webGL.Get("STATIC_DRAW"),
	)
}

func (r *RendererWeb) RenderQuad() {
	webGL := getWebGLContext()
	webGL.Call("drawArrays", webGL.Get("TRIANGLE_STRIP"), 0)
}
func (r *RendererWeb) RenderElements(mode PrimitiveMode, count, offset int) {
	webGL := getWebGLContext()
	webGL.Call("drawElements", r.MapPrimitiveMode(mode), count, webGL.Get("UNSIGNED_INT"), offset)
}

func (r *RendererWeb) RenderCubeMap(envTex Texture, cubeTex Texture) {
	webGL := getWebGLContext()
	envTexture := envTex.(*TextureWeb)
	cubeTexture := cubeTex.(*TextureWeb)
	textureSize := cubeTexture.width

	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_env)
	webGL.Call("viewport", 0, 0, textureSize, textureSize)

	webGL.Call("useProgram", r.panoramaToCubeMapShader.program)

	loc := r.panoramaToCubeMapShader.a["VertCoord"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, 0)

	jsData := js.Global().Get("Float32Array").New(8)
	vertices := []float32{-1, -1, 1, -1, -1, 1, 1, 1}
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), 32))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.vertexBuffer)
	webGL.Call("bufferData", webGL.Get("ARRAY_BUFFER"), jsData, webGL.Get("STATIC_DRAW"))

	loc, unit := r.panoramaToCubeMapShader.u["panorama"], r.panoramaToCubeMapShader.t["panorama"]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), envTexture.handle)
	webGL.Call("uniform1i", loc, unit)

	for i := 0; i < 6; i++ {
		webGL.Call("framebufferTexture2D",
			webGL.Get("FRAMEBUFFER"),
			webGL.Get("COLOR_ATTACHMENT0"),
			webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X").Int()+i,
			cubeTexture.handle,
			0,
		)
		webGL.Call("clear", webGL.Get("COLOR_BUFFER_BIT"))
		loc := r.panoramaToCubeMapShader.u["currentFace"]
		webGL.Call("uniform1i", loc, i)

		webGL.Call("drawArrays", webGL.Get("TRIANGLE_STRIP"), 0, 4)
	}
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), cubeTexture.handle)
	webGL.Call("generateMipmap", webGL.Get("TEXTURE_CUBE_MAP"))
}
func (r *RendererWeb) RenderFilteredCubeMap(distribution int32, cubeTex Texture, filteredTex Texture, mipmapLevel, sampleCount int32, roughness float32) {
	webGL := getWebGLContext()
	cubeTexture := cubeTex.(*TextureWeb)
	filteredTexture := filteredTex.(*TextureWeb)
	textureSize := filteredTexture.width
	currentTextureSize := textureSize >> mipmapLevel

	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_env)
	webGL.Call("viewport", 0, 0, currentTextureSize, currentTextureSize)

	webGL.Call("useProgram", r.cubemapFilteringShader.program)

	loc := r.cubemapFilteringShader.a["VertCoord"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, 0)

	jsData := js.Global().Get("Float32Array").New(8)
	vertices := []float32{-1, -1, 1, -1, -1, 1, 1, 1}
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), 32))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.vertexBuffer)
	webGL.Call("bufferData", webGL.Get("ARRAY_BUFFER"), jsData, webGL.Get("STATIC_DRAW"))

	loc, unit := r.cubemapFilteringShader.u["cubeMap"], r.cubemapFilteringShader.t["cubeMap"]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), cubeTexture.handle)
	webGL.Call("uniform1i", loc, unit)

	webGL.Call("uniform1i", r.cubemapFilteringShader.u["sampleCount"], sampleCount)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["distribution"], distribution)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["width"], currentTextureSize)
	webGL.Call("uniform1f", r.cubemapFilteringShader.u["roughness"], roughness)
	webGL.Call("uniform1f", r.cubemapFilteringShader.u["intensityScale"], 1)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["isLUT"], 0)

	for i := 0; i < 6; i++ {
		webGL.Call("framebufferTexture2D", webGL.Get("FRAMEBUFFER"), webGL.Get("COLOR_ATTACHMENT0"), webGL.Get("TEXTURE_CUBE_MAP_POSITIVE_X").Int()+i, filteredTexture.handle, mipmapLevel)

		webGL.Call("clear", webGL.Get("COLOR_BUFFER_BIT"))
		webGL.Call("uniform1i", r.cubemapFilteringShader.u["currentFace"], i)
		webGL.Call("drawArrays", webGL.Get("TRIANGLE_STRIP"), 0, 4)
	}
	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)
}
func (r *RendererWeb) RenderLUT(distribution int32, cubeTex Texture, lutTex Texture, sampleCount int32) {
	webGL := getWebGLContext()
	cubeTexture := cubeTex.(*TextureWeb)
	lutTexture := lutTex.(*TextureWeb)
	textureSize := lutTexture.width

	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo_env)
	webGL.Call("viewport", 0, 0, textureSize, textureSize)

	webGL.Call("useProgram", r.cubemapFilteringShader.program)

	loc := r.cubemapFilteringShader.a["VertCoord"]
	webGL.Call("enableVertexAttribArray", loc)
	webGL.Call("vertexAttribPointer", loc, 2, webGL.Get("FLOAT"), false, 0, 0)

	jsData := js.Global().Get("Float32Array").New(8)
	vertices := []float32{-1, -1, 1, -1, -1, 1, 1, 1}
	js.CopyBytesToJS(jsData, unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), 32))

	webGL.Call("bindBuffer", webGL.Get("ARRAY_BUFFER"), r.vertexBuffer)
	webGL.Call("bufferData", webGL.Get("ARRAY_BUFFER"), jsData, webGL.Get("STATIC_DRAW"))

	loc, unit := r.cubemapFilteringShader.u["cubeMap"], r.cubemapFilteringShader.t["cubeMap"]
	webGL.Call("activeTexture", webGL.Get("TEXTURE0").Int()+unit)
	webGL.Call("bindTexture", webGL.Get("TEXTURE_CUBE_MAP"), cubeTexture.handle)
	webGL.Call("uniform1i", loc, unit)

	webGL.Call("uniform1i", r.cubemapFilteringShader.u["sampleCount"], sampleCount)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["distribution"], distribution)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["width"], textureSize)
	webGL.Call("uniform1f", r.cubemapFilteringShader.u["roughness"], 0)
	webGL.Call("uniform1f", r.cubemapFilteringShader.u["intensityScale"], 1)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["currentFace"], 0)
	webGL.Call("uniform1i", r.cubemapFilteringShader.u["isLUT"], 1)

	webGL.Call("bindTexture", webGL.Get("TEXTURE_2D"), lutTexture.handle)

	webGL.Call("texImage2D",
		webGL.Get("TEXTURE_2D"),
		0,
		webGL.Get("RGBA32F"),
		lutTexture.width,
		lutTexture.height,
		0,
		webGL.Get("RGBA"),
		webGL.Get("FLOAT"),
		nil,
	)

	webGL.Call("framebufferTexture2D",
		webGL.Get("FRAMEBUFFER"),
		webGL.Get("COLOR_ATTACHMENT0"),
		webGL.Get("TEXTURE_2D"),
		lutTexture.handle,
		0,
	)

	webGL.Call("clear", webGL.Get("COLOR_BUFFER_BIT"))
	webGL.Call("drawArrays", webGL.Get("TRIANGLE_STRIP"), 0, 4)

	webGL.Call("bindFramebuffer", webGL.Get("FRAMEBUFFER"), r.fbo)
}
