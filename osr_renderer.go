package main

import (
	"C"
	"bytes"
	"encoding/binary"
	"log"
	"unsafe"

	"github.com/go-gl/gl/v4.4-compatibility/gl"
	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
)

type OsrRenderer struct {
	initialized_         bool
	texture_id_          uint32
	view_width_          int
	view_height_         int
	update_rect_         capi.CRectT
	popup_rect_          capi.CRectT
	original_popup_rect_ capi.CRectT
	spin_x_              float32
	spin_y_              float32

	show_update_rect bool
	background_color capi.CColorT
}

func NewOsrRenderer(show_update_rect bool, background_color_ capi.CColorT) *OsrRenderer {
	renderer := &OsrRenderer{
		show_update_rect: show_update_rect,
		background_color: background_color_,
	}

	return renderer
}

func (renderer *OsrRenderer) EnableGL(hwnd win32api.HWND) (win32api.HDC, win32api.HGLRC) {
	var pfd win32api.Pixelformatdescriptor

	// // Initialize Glow
	// if err := gl.Init(); err != nil {
	// 	log.Panicln("T39", err)
	// }

	hdc := win32api.GetDC(hwnd)
	pfd.Size = win32api.WORD(unsafe.Sizeof(pfd))
	pfd.Version = 1
	pfd.Flags = win32api.PfdDrawToWindow | win32api.PfdSupportOpengl | win32api.PfdDoublebuffer
	pfd.PixelType = win32api.PfdTypeRgba
	pfd.ColorBits = 24
	pfd.DepthBits = 16
	pfd.LayerType = win32api.PfdMainPlane
	format, err := win32api.ChoosePixelFormat(hdc, &pfd)
	if err != nil {
		log.Panicln("T522:", err)
	}
	err = win32api.SetPixelFormat(hdc, format, &pfd)
	if err != nil {
		log.Panicln("T526:", err)
	}
	hrc, err := win32api.WglCreateContext(hdc)
	if err != nil {
		log.Panicln("T530:", err)
	}
	if err := win32api.WglMakeCurrent(hdc, hrc); err != nil {
		log.Panicln("T534:", err)
	}
	defer func() {
		if err := win32api.WglMakeCurrent(0, 0); err != nil {
			log.Panicln("T534:", err)
		}
	}()

	renderer.Initialize()

	return hdc, hrc
}

func glMust(msg string, tail ...interface{}) {
	if err := gl.GetError(); err != gl.NO_ERROR {
		args := append([]interface{}{msg, err}, tail...)
		log.Panicln(args...)
	}
}

func (renderer *OsrRenderer) IsTransparent() bool {
	return capi.ColorGetA(renderer.background_color) == 0
}

func (renderer *OsrRenderer) Initialize() {

	if !renderer.initialized_ {
		// Initialize Glow
		if err := gl.Init(); err != nil {
			log.Panicln("T61", err)
		}
		gl.Hint(gl.POLYGON_SMOOTH_HINT, gl.NICEST)
		glMust("T551:")

		if renderer.IsTransparent() {
			gl.ClearColor(0, 0, 0, 0)
			glMust("T81:")
		} else {
			gl.ClearColor(float32(capi.ColorGetR(renderer.background_color))/255,
				float32(capi.ColorGetG(renderer.background_color))/255,
				float32(capi.ColorGetB(renderer.background_color))/255,
				1,
			)
			glMust("T88:")
		}
		// Necessary for non-power-of-2 textures to render correctly.
		gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		glMust("T79:")

		// Create the texture.
		gl.GenTextures(1, &renderer.texture_id_)
		glMust("T83:")

		if renderer.texture_id_ == 0 {
			log.Panicln("T85: texture_id_ is 0")
		}

		gl.BindTexture(gl.TEXTURE_2D, renderer.texture_id_)
		glMust("T89:")

		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		glMust("T92:")
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		glMust("T94:")
		gl.TexEnvf(gl.TEXTURE_ENV, gl.TEXTURE_ENV_MODE, gl.MODULATE)
		glMust("T96:")

	}
	renderer.initialized_ = true
}

func (renderer *OsrRenderer) OnPaint(
	browser *capi.CBrowserT,
	ctype capi.CPaintElementTypeT,
	dirtyRects []capi.CRectT,
	buffer unsafe.Pointer,
	width int,
	height int,
) {
	renderer.Initialize()

	if renderer.IsTransparent() {
		gl.Enable(gl.BLEND)
		glMust("T131:")
		defer func() {
			gl.Disable(gl.BLEND)
			glMust("T134:")
		}()
	}

	gl.Enable(gl.TEXTURE_2D)
	glMust("T137:")
	defer func() {
		gl.Disable(gl.TEXTURE_2D)
		glMust("T140:")
	}()

	gl.BindTexture(gl.TEXTURE_2D, renderer.texture_id_)
	glMust("T138:")

	if ctype == capi.PetView {
		old_width := renderer.view_width_
		old_height := renderer.view_height_

		renderer.view_width_ = width
		renderer.view_height_ = height
		if renderer.show_update_rect {
			renderer.update_rect_ = dirtyRects[0]
		}
		gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(renderer.view_width_))
		glMust("T163:")

		rendererRect := capi.CRectT{}
		rendererRect.SetX(0)
		rendererRect.SetY(0)
		rendererRect.SetWidth(renderer.view_width_)
		rendererRect.SetHeight(renderer.view_height_)
		if old_width != renderer.view_width_ || old_height != renderer.view_height_ ||
			(len(dirtyRects) == 1 &&
				dirtyRects[0] == rendererRect) {
			// line := 50*width*4
			// b := (*[1 << 30]byte)(buffer)[line:line+600]
			gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, 0)
			glMust("T178:")

			gl.PixelStorei(gl.UNPACK_SKIP_ROWS, 0)
			glMust("T181:")

			gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
				int32(renderer.view_width_), int32(renderer.view_height_), 0,
				gl.BGRA, gl.UNSIGNED_INT_8_8_8_8_REV, buffer)
			glMust("T185:")
		} else {
			for _, r := range dirtyRects {
				// line := 31 * width * 4
				// b := (*[1 << 30]byte)(buffer)[line : line+600]
				// log.Println("T191:", i, r, b)
				if r.X()+r.Width() > renderer.view_width_ {
					log.Panicln("T198:")
				}
				if r.Y()+r.Height() > renderer.view_height_ {
					log.Panicln("T192:")
				}
				gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, int32(r.X()))
				glMust("T195:")
				gl.PixelStorei(gl.UNPACK_SKIP_ROWS, int32(r.Y()))
				glMust("T197:")

				gl.TexSubImage2D(gl.TEXTURE_2D, 0,
					int32(r.X()), int32(r.Y()), int32(r.Width()), int32(r.Height()),
					gl.BGRA, gl.UNSIGNED_INT_8_8_8_8_REV, buffer)
				glMust("T204:")
			}
		}
	} else if ctype == capi.PetPopup &&
		renderer.popup_rect_.Width() > 0 && renderer.popup_rect_.Height() > 0 {
		skip_pixels := 0
		x := renderer.popup_rect_.X()
		skip_rows := 0
		y := renderer.popup_rect_.Y()
		w := width
		h := height

		if x < 0 {
			skip_pixels = -x
			x = 0
		}
		if y < 0 {
			skip_rows = -y
			y = 0
		}
		if x+w > renderer.view_width_ {
			w -= x + w - renderer.view_width_
		}
		if y+h > renderer.view_height_ {
			h -= y + h - renderer.view_height_
		}
		gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(width))
		glMust("T231:")
		gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, int32(skip_pixels))
		glMust("T233:")
		gl.PixelStorei(gl.UNPACK_SKIP_ROWS, int32(skip_rows))
		glMust("T235:")
		gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(x), int32(y), int32(w), int32(h),
			gl.BGRA, gl.UNSIGNED_INT_8_8_8_8_REV, buffer)
		glMust("T238:")
	}
}

type vertix struct {
	tu, tv  float32
	x, y, z float32
}

func (renderer *OsrRenderer) Render() {
	if renderer.view_width_ == 0 || renderer.view_height_ == 0 {
		return
	}
	if !renderer.initialized_ {
		log.Panicln("T260:")
	}

	goVertices := []vertix{
		{0, 1, -1, -1, 0},
		{1, 1, 1, -1, 0},
		{1, 0, 1, 1, 0},
		{0, 0, -1, 1, 0},
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, goVertices); err != nil {
		log.Panicln("T338:")
	}
	vertices := C.CBytes(buf.Bytes())

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	glMust("T275:")

	gl.MatrixMode(gl.MODELVIEW)
	glMust("T278:")
	gl.LoadIdentity()
	glMust("T281:")

	// Match GL units to screen coordinates.
	gl.Viewport(0, 0, int32(renderer.view_width_), int32(renderer.view_height_))
	glMust("T284:")
	gl.MatrixMode(gl.PROJECTION)
	glMust("T286:")

	// Draw the background gradient.
	gl.PushAttrib(gl.ALL_ATTRIB_BITS)
	glMust("T290:")
	// Don't check for errors until glEnd().
	gl.Begin(gl.QUADS)
	gl.Color4f(1, 0, 0, 1) // red
	gl.Vertex2f(-1, -1)
	gl.Vertex2f(1, -1)
	gl.Color4f(0, 0, 1, 1) // blue
	gl.Vertex2f(1, 1)
	gl.Vertex2f(-1, 1)
	gl.End()
	glMust("T300:")
	gl.PopAttrib()
	glMust("T302:")

	// Rotate the view based on the mouse spin.
	if renderer.spin_x_ != 0 {
		gl.Rotatef(-renderer.spin_x_, 1, 0, 0)
		glMust("T310:")
	}
	if renderer.spin_y_ != 0 {
		gl.Rotatef(-renderer.spin_y_, 0, 1, 0)
		glMust("T314:")
	}

	if renderer.IsTransparent() {
		// Alpha blending style. Texture values have premultiplied alpha.
		gl.BlendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
		glMust("T320:")

		// Enable alpha blending.
		gl.Enable(gl.BLEND)
		glMust("T324:")
	}
	// Enable 2D textures.
	gl.Enable(gl.TEXTURE_2D)
	glMust("T328:")
	// Draw the facets with the texture.
	if renderer.texture_id_ == 0 {
		log.Panicln("T332")
	}
	gl.BindTexture(gl.TEXTURE_2D, renderer.texture_id_)
	glMust("T335:")

	gl.InterleavedArrays(gl.T2F_V3F, 0, vertices)
	glMust("T337:")
	gl.DrawArrays(gl.QUADS, 0, 4)
	glMust("T339:")

	// Disable 2D textures.
	gl.Disable(gl.TEXTURE_2D)
	glMust("T343:")

	if renderer.IsTransparent() {
		// Disable alpha blending.
		gl.Disable(gl.BLEND)
		glMust("T348:")
	}

	// Draw a rectangle around the update region.
	if renderer.show_update_rect && !renderer.update_rect_.IsEmpty() {
		left := renderer.update_rect_.X()
		right := renderer.update_rect_.X() + renderer.update_rect_.Width()
		top := renderer.update_rect_.Y()
		bottom := renderer.update_rect_.Y() + renderer.update_rect_.Height()

		// Shrink the box so that left & bottom sides are drawn.
		left += 1
		bottom -= 1

		gl.PushAttrib(gl.ALL_ATTRIB_BITS)
		glMust("T363:")
		gl.MatrixMode(gl.PROJECTION)
		glMust("T365:")
		gl.PushMatrix()
		glMust("T367:")
		gl.LoadIdentity()
		glMust("T369:")
		gl.Ortho(0, float64(renderer.view_width_), float64(renderer.view_height_), 0, 0, 1)
		glMust("T371:")

		gl.LineWidth(1)
		glMust("T378:")
		gl.Color3f(1, 0, 0)
		glMust("T376:")
		// Don't check for errors until glEnd().
		gl.Begin(gl.LINE_STRIP)
		gl.Vertex2i(int32(left), int32(top))
		gl.Vertex2i(int32(right), int32(top))
		gl.Vertex2i(int32(right), int32(bottom))
		gl.Vertex2i(int32(left), int32(bottom))
		gl.Vertex2i(int32(left), int32(top))
		gl.End()
		glMust("T385:")

		gl.PopMatrix()
		glMust("T388:")
		gl.PopAttrib()
		glMust("T388:")
	}
}

func (renderer *OsrRenderer) SetSpin(spinX, spinY float32) {
	renderer.spin_x_ = spinX
	renderer.spin_y_ = spinY
}

func (renderer *OsrRenderer) IncrementSpin(spinDX, spinDY float32) {
	renderer.spin_x_ -= spinDX
	renderer.spin_y_ -= spinDY
}
