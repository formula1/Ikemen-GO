//go:build js

package main

import (
	"strconv"
	"syscall/js"
)

const (
	MAX_JOYSTICK_COUNT = 8
	MAX_BUTTON_COUNT   = 16
	MAX_AXIS_COUNT     = 8
)

type Input struct {
	jsGameID string
}

type Key int
type ModifierKey int32

const (
	KeyUnknown Key = iota

	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeySpace
	KeyQuote
	KeyComma
	KeyMinus
	KeyPeriod
	KeySlash
	KeyDigit0
	KeyDigit1
	KeyDigit2
	KeyDigit3
	KeyDigit4
	KeyDigit5
	KeyDigit6
	KeyDigit7
	KeyDigit8
	KeyDigit9
	KeySemicolon
	KeyEqual
	KeyBracketLeft
	KeyBackslash
	KeyBracketRight
	KeyBackquote
	KeyKeyA
	KeyKeyB
	KeyKeyC
	KeyKeyD
	KeyKeyE
	KeyKeyF
	KeyKeyG
	KeyKeyH
	KeyKeyI
	KeyKeyJ
	KeyKeyK
	KeyKeyL
	KeyKeyM
	KeyKeyN
	KeyKeyO
	KeyKeyP
	KeyKeyQ
	KeyKeyR
	KeyKeyS
	KeyKeyT
	KeyKeyU
	KeyKeyV
	KeyKeyW
	KeyKeyX
	KeyKeyY
	KeyKeyZ
	KeyCapsLock
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12

	KeyInsert
	KeyHome
	KeyPageUp
	KeyDelete
	KeyEnd
	KeyPageDown
	KeyArrowRight
	KeyArrowLeft
	KeyArrowDown
	KeyArrowUp
	KeyNumLock

	KeyNumpadDivide
	KeyNumpadMultiply
	KeyNumpadSubtract
	KeyNumpadAdd
	KeyNumpadEnter
	KeyNumpad1
	KeyNumpad2
	KeyNumpad3
	KeyNumpad4
	KeyNumpad5
	KeyNumpad6
	KeyNumpad7
	KeyNumpad8
	KeyNumpad9
	KeyNumpad0
	KeyNumpadDecimal

	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
	KeyContextMenu
)

var StringToJavascriptKeyCode = map[string]string{
	"RETURN":    "Enter",
	"ESCAPE":    "Escape",
	"BACKSPACE": "Backspace",
	"TAB":       "Tab",
	"SPACE":     "Space",
	"QUOTE":     "Quote",
	"COMMA":     "Comma",
	"MINUS":     "Minus",
	"PERIOD":    "Period",
	"SLASH":     "Slash",
	"0":         "Digit0",
	"1":         "Digit1",
	"2":         "Digit2",
	"3":         "Digit3",
	"4":         "Digit4",
	"5":         "Digit5",
	"6":         "Digit6",
	"7":         "Digit7",
	"8":         "Digit8",
	"9":         "Digit9",
	"SEMICOLON": "Semicolon",
	"EQUALS":    "Equal",
	"LBRACKET":  "BracketLeft",
	"BACKSLASH": "Backslash",
	"RBRACKET":  "BracketRight",
	"BACKQUOTE": "Backquote",
	"a":         "KeyA",
	"b":         "KeyB",
	"c":         "KeyC",
	"d":         "KeyD",
	"e":         "KeyE",
	"f":         "KeyF",
	"g":         "KeyG",
	"h":         "KeyH",
	"i":         "KeyI",
	"j":         "KeyJ",
	"k":         "KeyK",
	"l":         "KeyL",
	"m":         "KeyM",
	"n":         "KeyN",
	"o":         "KeyO",
	"p":         "KeyP",
	"q":         "KeyQ",
	"r":         "KeyR",
	"s":         "KeyS",
	"t":         "KeyT",
	"u":         "KeyU",
	"v":         "KeyV",
	"w":         "KeyW",
	"x":         "KeyX",
	"y":         "KeyY",
	"z":         "KeyZ",
	"CAPSLOCK":  "CapsLock",
	"F1":        "F1",
	"F2":        "F2",
	"F3":        "F3",
	"F4":        "F4",
	"F5":        "F5",
	"F6":        "F6",
	"F7":        "F7",
	"F8":        "F8",
	"F9":        "F9",
	"F10":       "F10",
	"F11":       "F11",
	"F12":       "F12",
	// "PRINTSCREEN": 	"",
	// "SCROLLLOCK": 	"",
	// "PAUSE": 				"",
	"INSERT":       "Insert",
	"HOME":         "Home",
	"PAGEUP":       "PageUp",
	"DELETE":       "Delete",
	"END":          "End",
	"PAGEDOWN":     "PageDown",
	"RIGHT":        "ArrowRight",
	"LEFT":         "ArrowLeft",
	"DOWN":         "ArrowDown",
	"UP":           "ArrowUp",
	"NUMLOCKCLEAR": "NumLock",

	"KP_DIVIDE":   "NumpadDivide",
	"KP_MULTIPLY": "NumpadMultiply",
	"KP_MINUS":    "NumpadSubtract",
	"KP_PLUS":     "NumpadAdd",
	"KP_ENTER":    "NumpadEnter",
	"KP_1":        "Numpad1",
	"KP_2":        "Numpad2",
	"KP_3":        "Numpad3",
	"KP_4":        "Numpad4",
	"KP_5":        "Numpad5",
	"KP_6":        "Numpad6",
	"KP_7":        "Numpad7",
	"KP_8":        "Numpad8",
	"KP_9":        "Numpad9",
	"KP_0":        "Numpad0",
	"KP_PERIOD":   "NumpadDecimal",

	"F13":  "F13",
	"F14":  "F14",
	"F15":  "F15",
	"F16":  "F16",
	"F17":  "F17",
	"F18":  "F18",
	"F19":  "F19",
	"F20":  "F20",
	"F21":  "F21",
	"F22":  "F22",
	"F23":  "F23",
	"F24":  "F24",
	"MENU": "ContextMenu",
}

var StringToKeyLUT = map[string]Key{
	"RETURN":    KeyEnter,
	"ESCAPE":    KeyEscape,
	"BACKSPACE": KeyBackspace,
	"TAB":       KeyTab,
	"SPACE":     KeySpace,
	"QUOTE":     KeyQuote,
	"COMMA":     KeyComma,
	"MINUS":     KeyMinus,
	"PERIOD":    KeyPeriod,
	"SLASH":     KeySlash,
	"0":         KeyDigit0,
	"1":         KeyDigit1,
	"2":         KeyDigit2,
	"3":         KeyDigit3,
	"4":         KeyDigit4,
	"5":         KeyDigit5,
	"6":         KeyDigit6,
	"7":         KeyDigit7,
	"8":         KeyDigit8,
	"9":         KeyDigit9,
	"SEMICOLON": KeySemicolon,
	"EQUALS":    KeyEqual,
	"LBRACKET":  KeyBracketLeft,
	"BACKSLASH": KeyBackslash,
	"RBRACKET":  KeyBracketRight,
	"BACKQUOTE": KeyBackquote,
	"a":         KeyKeyA,
	"b":         KeyKeyB,
	"c":         KeyKeyC,
	"d":         KeyKeyD,
	"e":         KeyKeyE,
	"f":         KeyKeyF,
	"g":         KeyKeyG,
	"h":         KeyKeyH,
	"i":         KeyKeyI,
	"j":         KeyKeyJ,
	"k":         KeyKeyK,
	"l":         KeyKeyL,
	"m":         KeyKeyM,
	"n":         KeyKeyN,
	"o":         KeyKeyO,
	"p":         KeyKeyP,
	"q":         KeyKeyQ,
	"r":         KeyKeyR,
	"s":         KeyKeyS,
	"t":         KeyKeyT,
	"u":         KeyKeyU,
	"v":         KeyKeyV,
	"w":         KeyKeyW,
	"x":         KeyKeyX,
	"y":         KeyKeyY,
	"z":         KeyKeyZ,
	"CAPSLOCK":  KeyCapsLock,
	"F1":        KeyF1,
	"F2":        KeyF2,
	"F3":        KeyF3,
	"F4":        KeyF4,
	"F5":        KeyF5,
	"F6":        KeyF6,
	"F7":        KeyF7,
	"F8":        KeyF8,
	"F9":        KeyF9,
	"F10":       KeyF10,
	"F11":       KeyF11,
	"F12":       KeyF12,
	// "PRINTSCREEN": 	"",
	// "SCROLLLOCK": 	"",
	// "PAUSE": 				"",
	"INSERT":       KeyInsert,
	"HOME":         KeyHome,
	"PAGEUP":       KeyPageUp,
	"DELETE":       KeyDelete,
	"END":          KeyEnd,
	"PAGEDOWN":     KeyPageDown,
	"RIGHT":        KeyArrowRight,
	"LEFT":         KeyArrowLeft,
	"DOWN":         KeyArrowDown,
	"UP":           KeyArrowUp,
	"NUMLOCKCLEAR": KeyNumLock,

	"KP_DIVIDE":   KeyNumpadDivide,
	"KP_MULTIPLY": KeyNumpadMultiply,
	"KP_MINUS":    KeyNumpadSubtract,
	"KP_PLUS":     KeyNumpadAdd,
	"KP_ENTER":    KeyNumpadEnter,
	"KP_1":        KeyNumpad1,
	"KP_2":        KeyNumpad2,
	"KP_3":        KeyNumpad3,
	"KP_4":        KeyNumpad4,
	"KP_5":        KeyNumpad5,
	"KP_6":        KeyNumpad6,
	"KP_7":        KeyNumpad7,
	"KP_8":        KeyNumpad8,
	"KP_9":        KeyNumpad9,
	"KP_0":        KeyNumpad0,
	"KP_PERIOD":   KeyNumpadDecimal,

	"F13":  KeyF13,
	"F14":  KeyF14,
	"F15":  KeyF15,
	"F16":  KeyF16,
	"F17":  KeyF17,
	"F18":  KeyF18,
	"F19":  KeyF19,
	"F20":  KeyF20,
	"F21":  KeyF21,
	"F22":  KeyF22,
	"F23":  KeyF23,
	"F24":  KeyF24,
	"MENU": KeyContextMenu,
}

var KeyToStringLUT = map[Key]string{}
var JavascriptKeyToKey = map[string]Key{}

func init() {
	for k, v := range StringToKeyLUT {
		KeyToStringLUT[v] = k
		if jskey, ok := StringToJavascriptKeyCode[k]; ok {
			JavascriptKeyToKey[jskey] = v
		} else {
			panic("Expected js key from " + k)
		}

	}
}

func StringToKey(s string) Key {
	if key, ok := StringToKeyLUT[s]; ok {
		return key
	}
	return KeyUnknown
}

func KeyToString(k Key) string {
	if s, ok := KeyToStringLUT[k]; ok {
		return s
	}
	return ""
}

func NewModifierKey(ctrl, alt, shift bool) (mod ModifierKey) {
	if ctrl {
		mod |= 1
	}
	if alt {
		mod |= 2
	}
	if shift {
		mod |= 4
	}
	return
}

var input *Input = newInput()

//export key_down_callback
func key_down_callback(key int32) {
	OnKeyPressed(Key(key), 0)
}

//export key_up_callback
func key_up_callback(key int32) {
	OnKeyReleased(Key(key), 0)
}

//export char_callback
func char_callback(ch uint32) {
	OnTextEntered(string(rune(ch)))
}

func newInput() *Input {

	js.Global().Get(jsGameInstance).Get("playerInput").Call(
		"keyEvent", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if len(args) < 3 {
				return nil
			}
			keyCode := args[0].String()
			modifier := args[1].Int()
			onOrOff := args[2].Bool()
			if key, ok := JavascriptKeyToKey[keyCode]; ok {
				if onOrOff {
					OnKeyPressed(Key(key), ModifierKey(modifier))
				} else {
					OnKeyReleased(Key(0), ModifierKey(modifier))
				}
			}
			return nil
		}),
	)

	return &Input{
		jsGameID: jsGameInstance,
	}
}

func (input *Input) GetMaxJoystickCount() int {
	return MAX_JOYSTICK_COUNT
}

func (input *Input) IsJoystickPresent(joy int) bool {
	result := js.Global().Get(input.jsGameID).Get("playerInput").Call(
		"gamepadExists", joy,
	)
	return bool(result.Bool())
}

func (input *Input) GetJoystickName(joy int) string {
	result := js.Global().Get(input.jsGameID).Get("playerInput").Call(
		"getGamepadName", joy,
	)
	if result.IsNull() {
		return "NOT CONNECTED"
	}
	return string(result.String())
}

func (input *Input) GetJoystickAxes(joy int) []float32 {
	result := js.Global().Get(input.jsGameID).Get("playerInput").Call(
		"getGamepadAxes", joy,
	)
	if result.IsNull() {
		return []float32{}
	}
	length := result.Length()
	arr := make([]float32, length)
	for i := 0; i < length; i++ {
		arr[i] = float32(result.Index(i).Float())
	}
	return arr
}

func (input *Input) GetJoystickButtons(joy int) []int32 {
	result := js.Global().Get(input.jsGameID).Get("playerInput").Call(
		"getGamepadButtons", joy,
	)
	if result.IsNull() {
		return []int32{}
	}
	length := result.Length()
	arr := make([]int32, length)
	for i := 0; i < length; i++ {
		arr[i] = int32(result.Index(i).Int())
	}
	return arr
}

// These methods is for darwin OS
// Since this is expected to run in wasm, we should ignore it
// see /src/system.go#327
func (input *Input) GetJoystickGUID(joy int) string {
	return ""
}

func (input *Input) GetJoystickIndices(guid string) []int {
	return []int{}
}

// From @leonkasovan's branch
func CheckAxisForDpad(joy int, axes *[]float32, base int) string {
	var s string = ""
	if (*axes)[0] > sys.cfg.Input.ControllerStickSensitivity { // right
		s = strconv.Itoa(2 + base)
	} else if -(*axes)[0] > sys.cfg.Input.ControllerStickSensitivity { // left
		s = strconv.Itoa(1 + base)
	}
	// fix OOB error that can happen on erroneous joysticks
	if len(*axes) < 2 {
		return s
	}
	if (*axes)[1] > sys.cfg.Input.ControllerStickSensitivity { // down
		s = strconv.Itoa(3 + base)
	} else if -(*axes)[1] > sys.cfg.Input.ControllerStickSensitivity { // up
		s = strconv.Itoa(base)
	}
	return s
}

// Adapted from @leonkasovan's branch (GLFW controllers are handled slightly differently depending on OS)
func CheckAxisForTrigger(joy int, axes *[]float32) string {
	return ""
}

type NoConnection struct{}

func (conn NoConnection) Read(b []byte) (int, error) {
	return 0, Error("Cannot Read")
}
func (conn NoConnection) Write(b []byte) (int, error) {
	return 0, Error("Cannot Write")
}
func (conn NoConnection) Close() error {
	return nil
}

type NoConnectionListener struct{}

func (conn NoConnectionListener) Close() error {
	return nil
}

func (listener NoConnectionListener) WaitForConnection() (NetConectionClient, error) {
	return nil, Error("Cannot Wait for Connection")
}

func CreateNetConnectionListener(args ...string) (NetConnectionListener, error) {
	return nil, Error("Cannot Listen for Connection")
}

func CreateNetConnection(args ...string) (NetConectionClient, error) {
	return nil, Error("Cannot Initiate a Connection")
}
