package ink

// Event/const definitions mirroring the real SDK's public surface. The concrete
// integer values are arbitrary (the real ones come from C headers) but internally
// consistent: a game compares e.State against these same constants, and the
// Harness constructs events with them, so only agreement matters.

import (
	"image"
	"image/color"
)

// Colours. The games only use these four greys, matching the real SDK.
var (
	Black     = color.Black
	White     = color.White
	DarkGray  = color.Gray{Y: 0x55}
	LightGray = color.Gray{Y: 0xaa}
)

// Default font names. In the real SDK these are C string macros; here we pick
// stable names and select a bold face when the name contains "Bold".
const (
	DefaultFont           = "PBSans"
	DefaultFontBold       = "PBSans-Bold"
	DefaultFontItalic     = "PBSans-Italic"
	DefaultFontBoldItalic = "PBSans-BoldItalic"
	DefaultFontMono       = "PBMono"
)

type KeyEvent struct {
	Key   Key
	State KeyState
}

type PointerEvent struct {
	image.Point
	State PointerState
}

type TouchEvent struct {
	image.Point
	State TouchState
}

type KeyState int

const (
	KeyStateDown KeyState = iota + 1
	KeyStatePress
	KeyStateUp
	KeyStateRelease
	KeyStateRepeat
)

type PointerState int

const (
	PointerUp PointerState = iota + 1
	PointerDown
	PointerMove
	PointerLong
	PointerHold
)

type TouchState int

const (
	TouchUp TouchState = iota + 1
	TouchDown
	TouchMove
)

// Key is a hardware key code.
type Key int

const (
	KeyBack Key = iota + 1
	KeyDelete
	KeyOk
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyMinus
	KeyPlus
	KeyMenu
	KeyMusic
	KeyPower
	KeyPrev
	KeyNext
	KeyPrev2
	KeyNext2
)

type Orientation int

const (
	Orientation0 Orientation = iota
	Orientation90
	Orientation180
	Orientation270
)
