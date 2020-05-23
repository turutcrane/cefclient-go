package main

import (
	"math"
	"log"
	"syscall"
	"unsafe"

	// "github.com/JamesHovious/w32"
	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
)

// #cgo pkg-config: cefingo
// #include "tests/cefclient/browser/resource.h"
import "C"

var rootWins = []*RootWindowWin{nil}

const (
	MAX_URL_LENGTH = 255
	BUTTON_WIDTH   = 72
	URLBAR_HEIGHT  = 24
)

func CreateRootWindow(
// settings *capi.CBrowserSettingsT,
// initially_hidden bool,
) {
	hInstance, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T31:", err)
	}
	window_title := "cefclient"
	window_class := "CEFCLIENT"

	background_color := CefColorSetARGB(255, 255, 255, 255)
	background_brush := win32api.CreateSolidBrush(
		RGB(CefColorGetR(background_color),
			CefColorGetG(background_color),
			CefColorGetB(background_color)),
	)
	RegisterRootClass(win32api.HINSTANCE(syscall.Handle(hInstance)), window_class, background_brush)

	dwExStyle := 0
	x := win32api.CwUsedefault
	y := win32api.CwUsedefault
	width := win32api.CwUsedefault
	height := win32api.CwUsedefault
	rootWindow := RootWindowWin{}
	rootWins = append(rootWins, &rootWindow)
	up := win32api.LPVOID(len(rootWins) - 1)

	wnd, err := win32api.CreateWindowEx(win32api.DWORD(dwExStyle),
		syscall.StringToUTF16Ptr(window_class),
		syscall.StringToUTF16Ptr(window_title),
		win32api.WsOverlappedwindow|win32api.WsClipchildren, //dwStyle
		x, y,
		width, height,
		0, // HWND
		0, // HMENU
		win32api.HINSTANCE(hInstance),
		up,
	)
	if wnd == 0 || err != nil {
		log.Panicln("T52: Failed to CreateWindowsEx", wnd, err)
	}
	win32api.ShowWindow(rootWindow.hwnd_, win32api.SwShownormal)
	if !win32api.UpdateWindow(rootWindow.hwnd_) {
		log.Panicln("T63: ShowWindow")
	}
}

func RGB(r, g, b uint32) win32api.COLORREF {
	return win32api.COLORREF(r | g<<8 | b<<16)
}

func CefColorSetARGB(a, r, g, b int) capi.CColorT {
	return capi.CColorT(a<<24 | r<<16 | g<<8 | b)
}

func CefColorGetA(c capi.CColorT) uint32 {
	return (uint32(c) >> 24) & 0xff
}

func CefColorGetR(c capi.CColorT) uint32 {
	return (uint32(c) >> 16) & 0xff
}

func CefColorGetG(c capi.CColorT) uint32 {
	return (uint32(c) >> 8) & 0xff
}

func CefColorGetB(c capi.CColorT) uint32 {
	return uint32(c) & 0xff
}

var class_regsitered bool

func RegisterRootClass(hInstance win32api.HINSTANCE, window_class string, background_brush win32api.HBRUSH) {
	if class_regsitered {
		return
	}
	class_regsitered = true

	icon, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(C.IDI_CEFCLIENT)) // w32.IDI_APPLICATION
	if err != nil {
		log.Panicln("T105: LoadIcon", err)
	}
	// icon := win32api.HICON(w32.LoadIcon(w32.HINSTANCE(hInstance), w32.MakeIntResource(C.IDI_CEFCLIENT))) // w32.IDI_APPLICATION
	// log.Panicln("T114: LoadIcon", icon, w32.GetLastError())

	iconSm, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(C.IDI_SMALL))
	if err != nil {
		log.Panicln("T109: LoadIcon Sm", err)
	}
	cursor, err := win32api.LoadCursor(0, win32api.IdcArrow)
	if err != nil {
		log.Panicln("T113: LoadCursor", err)
	}
	wndClass := win32api.Wndclassex{
		Size:       uint32(unsafe.Sizeof(win32api.Wndclassex{})),
		Style:      win32api.CsHredraw | win32api.CsVredraw,
		WndProc:    syscall.NewCallback(RootWndProc),
		ClsExtra:   0,
		WndExtra:   0,
		Instance:   hInstance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 0,
		MenuName:   win32api.MakeIntResource(C.IDC_CEFCLIENT),
		ClassName:  syscall.StringToUTF16Ptr(window_class),
		IconSm:     iconSm,
	}
	if win32api.RegisterClassEx(&wndClass) == 0 {
		log.Panicln("T109: Can not Register Class", window_class)
	}
}

type RootWindowWin struct {
	hwnd_                                 win32api.HWND
	back_hwnd_                            win32api.HWND
	find_message_id_                      win32api.UINT
	called_enable_non_client_dpi_scaling_ bool
}

func RootWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	var self *RootWindowWin
	if message != win32api.WmNccreate {
		win32api.SetLastError(0)
		up, err := win32api.GetWindowLongPtr(hWnd, win32api.GwlpUserdata)
		if up == 0 {
			if err != nil {
				log.Println("T159: GetWindowLongPtr GwlpUserdata", hWnd, err)
			}
			return win32api.DefWindowProc(hWnd, message, wParam, lParam)
		}
		self = rootWins[up]
		if self.hwnd_ != hWnd {
			log.Panicln("T93: hwnd missmatch!", self.hwnd_, hWnd)
		}
	}
	if self != nil && message == self.find_message_id_ {
		// lpfr := w32.LPFINDREPLACE(lParam)
		// self->OnFindEvent()
		log.Panicln("T102: not impremented")
		return 0
	}

	switch message {
	case win32api.WmNccreate:
		cs := (*win32api.Createstruct)(unsafe.Pointer(lParam))
		self := rootWins[cs.CreateParams]
		if self == nil {
			log.Panicln("T111: self not set")
		}

		r, err := win32api.SetWindowLongPtr(hWnd, win32api.GwlpUserdata, uintptr(cs.CreateParams))
		if err != nil {
			log.Panicln("T186: SetWindowLongPtr", r, hWnd, err)
		}
		self.hwnd_ = hWnd
		self.OnNCCreate(cs)

	case win32api.WmCreate:
		cs := (*win32api.Createstruct)(unsafe.Pointer(lParam))
		self.OnCreate(cs)
	}

	return win32api.DefWindowProc(hWnd, message, wParam, lParam)
}

func (self *RootWindowWin) OnNCCreate(cs *win32api.Createstruct) {
	if IsProcessPerMonitorDpiAware() {
		enable_non_client_dpi_scaling, err := win32api.EnableNonClientDpiScaling(self.hwnd_)
		if err != nil {
			log.Panicln("T191:", err)
		}
		self.called_enable_non_client_dpi_scaling_ = enable_non_client_dpi_scaling
	}
}

var processPerMonitorDpiAware *bool

func IsProcessPerMonitorDpiAware() bool {
	if processPerMonitorDpiAware == nil {
		var dpiAwareness win32api.ProcessDpiAwareness
		var aBool bool
		processPerMonitorDpiAware = &aBool

		hresult := win32api.GetProcessDpiAwareness(0, &dpiAwareness)
		if hresult == win32api.SOk {
			*processPerMonitorDpiAware = true
		}
	}
	return *processPerMonitorDpiAware
}

func (self *RootWindowWin) OnCreate(cs *win32api.Createstruct) {
	hInstance := cs.Instance

	var rect win32api.RECT
	if win32api.GetClientRect(self.hwnd_, &rect) {
		log.Panicln("T221: GetClientRect")
	}
	log.Printf("T155: OnCreate")

	// if (with_controls_) skip
	x_offset := 0
	button_width := GetButtonWidth(self.hwnd_)
	urlbar_height := GetURLBarHeight(self.hwnd_)
	// with_controles_
	h, err := win32api.CreateWindowEx(
		win32api.DWORD(0),
		syscall.StringToUTF16Ptr("BACK"),
		syscall.StringToUTF16Ptr("Back"),
		win32api.WsChild|win32api.WsVisible|win32api.BsPushbutton|win32api.WsDisabled,
		x_offset, 0,
		button_width, urlbar_height,
		self.hwnd_,
		win32api.HMENU(C.IDC_NAV_BACK),
		hInstance,
		0,
	)
	if err != nil {
		log.Panicln("T242: Create Button", err)
	}
	self.back_hwnd_ = h
	x_offset += button_width

}

func GetButtonWidth(hwnd win32api.HWND) int {
	return LogicalToDevice(BUTTON_WIDTH, GetWindowScaleFactor(hwnd))
}

func GetURLBarHeight(hwnd win32api.HWND) int {
	return LogicalToDevice(URLBAR_HEIGHT, GetWindowScaleFactor(hwnd))
}

const (
	DPI_1X = 96.0
)

func GetWindowScaleFactor(hwnd win32api.HWND) float32 {
	if hwnd != 0 && IsProcessPerMonitorDpiAware() {
		return float32(win32api.GetDpiForWindow(hwnd)) / DPI_1X
	}
	return GetDeviceScaleFactor()
}

func LogicalToDevice(value int, device_scale_factor float32) int {
	scaled_val := float32(value) * device_scale_factor
	return int(math.Floor(float64(scaled_val)))
}

var scale_factor float32 = 1.0
var initialized bool = false

func GetDeviceScaleFactor() float32 {

	if !initialized {
		// This value is safe to cache for the life time of the app since the user
		// must logout to change the DPI setting. This value also applies to all
		// screens.
		screen_dc := win32api.GetDC(0)
		dpi_x := win32api.GetDeviceCaps(screen_dc, win32api.Logpixelsx)
		scale_factor = float32(dpi_x) / DPI_1X
		win32api.ReleaseDC(0, screen_dc)
		initialized = true
	}

	return scale_factor
}
