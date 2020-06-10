package main

import (
	"log"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

// #include "tests/cefclient/browser/resource.h"
import "C"

var rootWins = []*RootWindowWin{nil} // index 0 is not usable.
var rootWinMap sync.Map              // hwnd -> *RootWindowWin

func NewRootWindowWin(settings *capi.CBrowserSettingsT) (key int, rootWindow *RootWindowWin) {
	rootWindow = &RootWindowWin{}
	rootWindow.with_controls_ = true
	rootWins = append(rootWins, rootWindow)
	up := len(rootWins) - 1
	rootWindow.browser_settings_ = settings

	rootWindow.browser_window_ = NewBrowserWindow(rootWindow)
	return up, rootWindow
}

func SetRootWin(hwnd win32api.HWND, rootWin *RootWindowWin) {
	rootWinMap.Store(hwnd, rootWin)
}

func GetRootWin(hwnd win32api.HWND) (rootWin *RootWindowWin, exist bool) {
	if r, ok := rootWinMap.Load(hwnd); ok {
		rootWin = r.(*RootWindowWin)
		exist = ok
	}

	return rootWin, exist
}

type RootWindowWin struct {
	with_controls_                        bool
	browser_settings_                     *capi.CBrowserSettingsT
	browser_window_                       *BrowserWindow
	hwnd_                                 win32api.HWND
	font_                                 win32api.HFONT
	font_height_                          int
	back_hwnd_                            win32api.HWND
	forward_hwnd_                         win32api.HWND
	reload_hwnd_                          win32api.HWND
	stop_hwnd_                            win32api.HWND
	edit_hwnd_                            win32api.HWND
	find_message_id_                      win32api.UINT
	edit_wndproc_old_                     win32api.WNDPROC
	called_enable_non_client_dpi_scaling_ bool
}

func RootWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	var self *RootWindowWin
	if message != win32const.WmNccreate {
		var ok bool
		self, ok = GetRootWin(hWnd)
		if !ok {
			log.Println("T159: GetWindowLongPtr GwlpUserdata", hWnd)
			return win32api.DefWindowProc(hWnd, message, wParam, lParam)
		}
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
	case win32const.WmPaint:
		self.OnPaint()
		return 0

	case win32const.WmActivate:
		self.OnActivate(win32api.LOWORD(wParam) != win32const.WaInactive)
		// Allow DefWindowProc to set keyboard focus.

	case win32const.WmSetfocus:
		self.OnFocus()
		return 0

	case win32const.WmSize:
		self.OnSize(wParam == win32const.SizeMinimized)

	case win32const.WmNccreate:
		cs := win32api.ToPCreatestruct(lParam)
		self := rootWins[cs.CreateParams]
		if self == nil {
			log.Panicln("T111: self not set")
		}

		SetRootWin(hWnd, self)

		self.hwnd_ = hWnd
		self.OnNCCreate(cs)

	case win32const.WmCreate:
		cs := win32api.ToPCreatestruct(lParam)
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

func (self *RootWindowWin) OnCreate(cs *win32api.Createstruct) {
	hInstance := cs.Instance

	var rect win32api.Rect
	_, err := win32api.GetClientRect(self.hwnd_, &rect)
	if err != nil {
		log.Panicln("T221: GetClientRect")
	}
	log.Printf("T155: OnCreate")

	if self.with_controls_ {
		// if (with_controls_) skip
		x_offset := 0
		button_width := GetButtonWidth(self.hwnd_)
		urlbar_height := GetURLBarHeight(self.hwnd_)
		// with_controles_
		back_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Back"),
			win32const.WsChild|win32const.WsVisible|win32const.BsPushbutton|win32const.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(C.IDC_NAV_BACK),
			hInstance, 0,
		)
		if err != nil {
			log.Panicln("T242: Create Button", err)
		}
		self.back_hwnd_ = back_hwnd_
		x_offset += button_width

		forward_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Forward"),
			win32const.WsChild|win32const.WsVisible|win32const.BsPushbutton|win32const.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(C.IDC_NAV_FORWARD),
			hInstance, 0,
		)
		if err != nil {
			log.Panicln("T242: Create Button", err)
		}
		self.forward_hwnd_ = forward_hwnd_
		x_offset += button_width

		reload_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Reload"),
			win32const.WsChild|win32const.WsVisible|win32const.BsPushbutton|win32const.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(C.IDC_NAV_FORWARD),
			hInstance, 0,
		)
		if err != nil {
			log.Panicln("T242: Create Button", err)
		}
		self.reload_hwnd_ = reload_hwnd_
		x_offset += button_width

		stop_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Stop"),
			win32const.WsChild|win32const.WsVisible|win32const.BsPushbutton|win32const.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(C.IDC_NAV_FORWARD),
			hInstance, 0,
		)
		if err != nil {
			log.Panicln("T242: Create Button", err)
		}
		self.stop_hwnd_ = stop_hwnd_
		x_offset += button_width

		edit_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("EDIT"), nil,
			win32const.WsChild|win32const.WsVisible|win32const.WsBorder|
				win32const.EsLeft|win32const.EsAutovscroll|win32const.EsAutohscroll|win32const.WsDisabled,
			x_offset, 0, int(rect.Right)-button_width*4, urlbar_height,
			self.hwnd_, 0,
			hInstance, 0,
		)
		if err != nil {
			log.Panicln("T242: Create Button", err)
		}
		self.edit_hwnd_ = edit_hwnd_
		x_offset += button_width

		// // Override the edit control's window procedure.
		self.edit_wndproc_old_ = SetWndProc(edit_hwnd_, EditWndProc)

		// // Associate |this| with the edit window.
		// SetUserDataPtr(edit_hwnd_, this)
		SetRootWin(edit_hwnd_, self)

		rect.Top += win32api.LONG(urlbar_height)
	} else {
		win32api.SetMenu(self.hwnd_, 0)
	}

	// device_scale_factor := GetWindowScaleFactor(self.hwnd_)
	// if (with_osr_) {
	// 	self.browser_window_.SetDeviceScaleFactor(device_scale_factor)
	// }

	r := &capi.CRectT{}
	r.SetX(int(rect.Left))
	r.SetY(int(rect.Top))
	r.SetWidth(int(rect.Right - rect.Left))
	r.SetHeight(int(rect.Bottom - rect.Top))
	// parentHwnd := capi.CWindowHandleT(unsafe.Pointer(uintptr(self.hwnd_)))
	parentHwnd := capi.ToCWindowHandleT(syscall.Handle(self.hwnd_))
	self.browser_window_.CreateBrowser(parentHwnd, r, self.browser_settings_, nil, nil) // delegate が PDF extension を許可している)
}

func (self *RootWindowWin) OnPaint() {
	var ps win32api.Paintstruct
	win32api.BeginPaint(self.hwnd_, &ps)
	win32api.EndPaint(self.hwnd_, &ps)
}

func (sef *RootWindowWin) OnActivate(active bool) {
}

func (self *RootWindowWin) OnFocus() {
	if self.browser_window_ != nil && win32api.IsWindowEnabled(self.hwnd_) {
		self.browser_window_.SetFocus(true)
	}
}

func (self *RootWindowWin) OnSize(minimized bool) {
	if minimized {
		if self.browser_window_ != nil {
			self.browser_window_.Hide()
		}
		return
	}

	if self.browser_window_ != nil {
		self.browser_window_.Show()
	}

	var rect win32api.Rect
	_, err := win32api.GetClientRect(self.hwnd_, &rect)
	if err != nil {
		log.Panicln("T269: GetClientRect")
	}

	if self.with_controls_ && self.edit_hwnd_ != 0 {
		button_width := GetButtonWidth(self.hwnd_)
		urlbar_height := GetURLBarHeight(self.hwnd_)
		font_height := LogicalToDevice(14, GetWindowScaleFactor(self.hwnd_))

		if font_height != self.font_height_ {
			font_height = self.font_height_
			if self.font_ != 0 {
				win32api.DeleteObject(win32api.HGDIOBJ(self.font_))
			}

			self.font_ = win32api.CreateFont(-font_height, 0, 0, 0,
				win32const.FwDontcare, false, false, false,
				win32const.DefaultCharset,
				win32const.OutDefaultPrecis,
				win32const.ClipDefaultPrecis,
				win32const.DefaultQuality,
				win32const.DefaultPitch|win32const.FfDontcare,
				syscall.StringToUTF16Ptr("Arial"),
			)

			win32api.SendMessage(self.back_hwnd_, win32const.WmSetfont, win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.forward_hwnd_, win32const.WmSetfont, win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.reload_hwnd_, win32const.WmSetfocus, win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.stop_hwnd_, win32const.WmSetfont, win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.edit_hwnd_, win32const.WmSetfont, win32api.WPARAM(self.font_), 1)
		}
		rect.Top += win32api.LONG(urlbar_height)
		x_offset := int(rect.Left)

		var browser_hwnd win32api.HWND
		if self.browser_window_ != nil {
			browser_hwnd = self.browser_window_.GetWindowHandle()
		}

		var hdwp win32api.HDWP
		if browser_hwnd != 0 {
			hdwp, err = win32api.BeginDeferWindowPos(6)
		} else {
			hdwp, err = win32api.BeginDeferWindowPos(5)
		}
		if err != nil {
			log.Panicln("T317:", err)
		}

		hdwp, err = win32api.DeferWindowPos(hdwp, self.back_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32const.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.forward_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32const.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.reload_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32const.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.stop_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32const.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.edit_hwnd_, 0, x_offset, 0, int(rect.Right)-x_offset, urlbar_height, win32const.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		if browser_hwnd != 0 {
			hdwp, err = win32api.DeferWindowPos(
				hdwp, browser_hwnd, 0,
				int(rect.Left), int(rect.Top),
				int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
				win32const.SwpNozorder,
			)
		}
		_, err = win32api.EndDeferWindowPos(hdwp)
		if err != nil {
			log.Panicln("T359:", err)
		}
	} else if self.browser_window_ != nil {
		self.browser_window_.SetBound(0, 0, uint32(rect.Right), uint32(rect.Bottom))
	}
}

func (self *RootWindowWin) GetBrowser() *capi.CBrowserT {
	return self.browser_window_.browser_
}

const MaxUrlLength = 255

func EditWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	self, ok := GetRootWin(hWnd)
	if !ok {
		log.Panicln("T386:", hWnd)
	}
	if hWnd != self.edit_hwnd_ {
		log.Panicln("T391: edit_hwnd_ not match", hWnd, self.edit_hwnd_)
	}

	switch message {
	case win32const.WmChar:
		if wParam == win32const.VkReturn {
			browser := self.GetBrowser()
			urlstr := [MaxUrlLength + 1]uint16{}
			urlstr[0] = MaxUrlLength
			sp := win32api.LPARAM(uintptr(unsafe.Pointer(&urlstr)))
			result := win32api.SendMessage(hWnd, win32const.EmGetline, 0, sp)
			log.Println("T242:", result)
			if result > 0 {
				runes := utf16.Decode(urlstr[0:result])
				url := string(runes)
				log.Println("T245:", url)
				browser.GetMainFrame().LoadUrl(url)
			}
			return 0
		}
	case win32const.WmNcdestroy:
	}

	return win32api.CallWindowProc(self.edit_wndproc_old_, hWnd, message, wParam, lParam)
}
