package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/win32api"
)

type RootWindowWin struct {
	key         int
	initial_url string

	with_controls_    bool
	always_on_top_    bool
	with_osr_         bool
	with_extension_   bool
	no_activate_      bool
	is_popup_         bool
	start_rect_       win32api.Rect
	browser_settings_ *capi.CBrowserSettingsT
	browser_window_   BrowserWindow
	hwnd_             win32api.HWND
	draggable_region_ win32api.HRGN
	font_             win32api.HFONT
	font_height_      int
	back_hwnd_        win32api.HWND
	forward_hwnd_     win32api.HWND
	reload_hwnd_      win32api.HWND
	stop_hwnd_        win32api.HWND

	edit_hwnd_        win32api.HWND
	edit_wndproc_old_ win32api.WNDPROC

	find_hwnd_            win32api.HWND
	find_message_id_      win32api.UINT
	find_wndproc_old_     win32api.WNDPROC
	find_state_           win32api.Findreplace
	find_buff_            [80]uint16
	find_what_last_       string
	find_next_            bool
	find_match_case_last_ bool

	called_enable_non_client_dpi_scaling_ bool

	window_destroyed_  bool
	browser_destroyed_ bool
}

func (rw *RootWindowWin) WithExtesion() bool {
	return rw.with_extension_
}

type DeviceScaleFactorer interface {
	SetDeviceScaleFactor(float32)
	GetDeviceScaleFactor() float32
}
type BrowserWindow interface {
	// GetWindowHandle() win32api.HWND
	CreateBrowser(
		initial_url string,
		parentHwnd win32api.HWND,
		rect capi.CRectT,
		settings *capi.CBrowserSettingsT,
		extra_info *capi.CDictionaryValueT,
		request_context *capi.CRequestContextT,
	)
	SetFocus(focus bool)
	Hide()
	Show()
	IsClosing() bool
	GetCBrowserT() *capi.CBrowserT
	GetCClientT() *capi.CClientT
	GetResourceManager() *ResourceManager
	IsOsr() bool
	ShowPopup(hwnd_ win32api.HWND, rect capi.CRectT)
}

func (rw *RootWindowWin) Init(
	config ClientConfig,
	is_popup bool,
	rect win32api.Rect,
	settings *capi.CBrowserSettingsT,
) {
	rw.initial_url = config.main_url
	rw.start_rect_ = rect
	rw.with_osr_ = config.use_windowless_rendering
	rw.always_on_top_ = config.always_on_top
	rw.no_activate_ = config.no_activate

	rw.draggable_region_ = win32api.CreateRectRgn(0, 0, 0, 0)
	rw.with_controls_ = config.with_controls
	rw.is_popup_ = is_popup
	rw.browser_settings_ = settings
	if rw.with_osr_ {
		rw.browser_window_ = NewBrowserWindowOsr(
			rw,
			mainConfig.show_update_rect,
			mainConfig.external_begin_frame_enabled,
			mainConfig.windowless_frame_rate,
			mainConfig.background_color,
		)
	} else {
		capi.Logln("T111:")
		rw.browser_window_ = NewBrowserWindowStd(rw)
	}

	return
}

func (rw *RootWindowWin) CreateWindow(
// initially_hidden bool,
) {
	hInstance, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T31:", err)
	}
	window_title := "cefclient"
	window_class := "CEFCLIENT"

	background_color := mainConfig.background_color
	background_brush := win32api.CreateSolidBrush(
		RGB(capi.ColorGetR(background_color),
			capi.ColorGetG(background_color),
			capi.ColorGetB(background_color)),
	)
	RegisterRootClass(win32api.HINSTANCE(hInstance), window_class, background_brush)
	r, err := win32api.RegisterWindowMessage(syscall.StringToUTF16Ptr(win32api.Findmsgstring))
	if err != nil {
		log.Panicln("T93:", err)
	}
	rw.find_message_id_ = r

	dwStyle := win32api.DWORD(win32api.WsOverlappedwindow | win32api.WsClipchildren)

	dwExStyle := win32api.DWORD(0)
	if rw.always_on_top_ {
		dwExStyle |= win32api.WsExTopmost
	}
	if rw.no_activate_ {
		dwExStyle |= win32api.WsExNoactivate
	}

	var x, y, width, height int
	if win32api.IsRectEmpty(&rw.start_rect_) {
		x = win32api.CwUsedefault
		y = win32api.CwUsedefault
		width = win32api.CwUsedefault
		height = win32api.CwUsedefault
	} else {
		if err := win32api.AdjustWindowRectEx(&rw.start_rect_, dwStyle, true, dwExStyle); err != nil {
			log.Panicln("T85:", err)
		}
	}

	wnd, err := win32api.CreateWindowEx(dwExStyle,
		syscall.StringToUTF16Ptr(window_class),
		syscall.StringToUTF16Ptr(window_title),
		dwStyle,
		x, y,
		width, height,
		0, // HWND
		0, // HMENU
		win32api.HINSTANCE(hInstance),
		win32api.LPVOID(rw.key),
	)
	if wnd == 0 || err != nil || wnd != rw.hwnd_ {
		log.Panicln("T52: Failed to CreateWindowsEx", wnd, err, rw.hwnd_)
	}

	win32api.ShowWindow(rw.hwnd_, win32api.SwShownormal)
	if !win32api.UpdateWindow(rw.hwnd_) {
		log.Panicln("T63: ShowWindow")
	}
}

func RootWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	var self *RootWindowWin
	msgId := win32api.MessageId(message)
	if msgId != win32api.WmNccreate {
		var ok bool
		self, ok = windowManager.GetRootWin(hWnd)
		if !ok {
			return win32api.DefWindowProc(hWnd, message, wParam, lParam)
		}
		if self.hwnd_ != hWnd {
			log.Panicln("T93: hwnd missmatch!", self.hwnd_, hWnd)
		}
	}
	if self != nil && message == self.find_message_id_ {
		// lpfr := w32.LPFINDREPLACE(lParam)
		if uintptr(lParam) != uintptr(unsafe.Pointer(&self.find_state_)) {
			log.Panicln("T155: lParam not match", lParam, self.find_state_)
		}

		self.OnFindEvent()
		return 0
	}

	switch msgId {
	case win32api.WmCommand:
		if self.OnCommand(win32api.UINT(win32api.LOWORD(wParam))) {
			return 0
		}

	case win32api.WmGetobject:
		// Only the lower 32 bits of lParam are valid when checking the object id
		// because it sometimes gets sign-extended incorrectly (but not always).
		obj_id := win32api.DWORD(lParam)

		// Accessibility readers will send an OBJID_CLIENT message.
		if win32api.DWORD(0xffffffff&win32api.ObjidClient) == obj_id {
			h := self.GetBrowser().GetHost()
			defer h.Unref()
			if self.GetBrowser() != nil && h != nil {
				h.SetAccessibilityState(capi.StateEnabled)
			}
		}

	case win32api.WmPaint:
		self.OnPaint()
		return 0

	case win32api.WmActivate:
		self.OnActivate(win32api.LOWORD(wParam) != win32api.WaInactive)
		// Allow DefWindowProc to set keyboard focus.

	case win32api.WmSetfocus:
		self.OnFocus()
		return 0

	case win32api.WmSize:
		self.OnSize(wParam == win32api.SizeMinimized)

	case win32api.WmMoving, win32api.WmMove:
		self.OnMove()
		return 0

	case win32api.WmDpichanged:
		self.OnDpiChanged(wParam, lParam)

	case win32api.WmErasebkgnd:
		if !self.OnEraseBkgnd() {
			return 0 // Don't erase the background.
		}

	case win32api.WmEntermenuloop:
		if wParam == 0 {
			// Entering the menu loop for the application menu.
			capi.SetOsmodalLoop(true)
		}

	case win32api.WmExitmenuloop:
		if wParam == 0 {
			// Exiting the menu loop for the application menu.
			capi.SetOsmodalLoop(false)
		}

	case win32api.WmClose:
		if self.OnClose() {
			return 0
		}

	case win32api.WmNchittest:
		hit := win32api.DefWindowProc(hWnd, message, wParam, lParam)
		if hit == win32api.Htclient {
			points := win32api.Makepoints(lParam)
			point := win32api.Point{X: win32api.LONG(points.X), Y: win32api.LONG(points.Y)}
			win32api.ScreenToClient(hWnd, &point)
			if win32api.PtInRegion(self.draggable_region_, int(point.X), int(point.Y)) {
				// If cursor is inside a draggable region return HTCAPTION to allow
				// dragging.
				return win32api.Htcaption
			}
		}
		return hit

	case win32api.WmNccreate:
		cs := win32api.ToPCreatestruct(lParam)
		self := windowManager.Lookup(int(cs.CreateParams))
		if self == nil {
			log.Panicln("T111: self not set")
		}

		// Associate |self| with the main window.
		// SetUserDataPtr(hWnd, self);
		windowManager.SetRootWin(hWnd, self)

		self.hwnd_ = hWnd
		self.OnNCCreate(cs)

	case win32api.WmCreate:
		cs := win32api.ToPCreatestruct(lParam)
		self.OnCreate(cs)

	case win32api.WmNcdestroy:
		// win32api.SetUserDataPtr(wWnd, nil)
		windowManager.RemoveRootWin(hWnd)
		self.hwnd_ = 0
		self.OnDestroyed()
	}

	return win32api.DefWindowProc(hWnd, message, wParam, lParam)
}

func (self *RootWindowWin) OnNCCreate(cs *win32api.Createstruct) {
	if IsProcessPerMonitorDpiAware() {
		if err := win32api.EnableNonClientDpiScaling(self.hwnd_); err != nil {
			log.Panicln("T191:", err)
		}
		self.called_enable_non_client_dpi_scaling_ = true
	}
}

func (self *RootWindowWin) OnCreate(cs *win32api.Createstruct) {
	hInstance := cs.Instance

	var rect win32api.Rect
	if err := win32api.GetClientRect(self.hwnd_, &rect); err != nil {
		log.Panicln("T221: GetClientRect")
	}

	if self.with_controls_ {
		// if (with_controls_) skip
		x_offset := 0
		button_width := GetButtonWidth(self.hwnd_)
		urlbar_height := GetURLBarHeight(self.hwnd_)
		// with_controles_
		if back_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Back"),
			win32api.WsChild|win32api.WsVisible|win32api.BsPushbutton|win32api.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(IdcNavBack),
			hInstance, 0,
		); err == nil {
			self.back_hwnd_ = back_hwnd_
		} else {
			log.Panicln("T242: Create Button", err)
		}
		x_offset += button_width

		if forward_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Forward"),
			win32api.WsChild|win32api.WsVisible|win32api.BsPushbutton|win32api.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(IdcNavForward),
			hInstance, 0,
		); err == nil {
			self.forward_hwnd_ = forward_hwnd_
		} else {
			log.Panicln("T242: Create Button", err)
		}
		x_offset += button_width

		if reload_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Reload"),
			win32api.WsChild|win32api.WsVisible|win32api.BsPushbutton|win32api.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(IdcNavReload),
			hInstance, 0,
		); err == nil {
			self.reload_hwnd_ = reload_hwnd_
		} else {
			log.Panicln("T242: Create Button", err)
		}
		x_offset += button_width

		if stop_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("BUTTON"), syscall.StringToUTF16Ptr("Stop"),
			win32api.WsChild|win32api.WsVisible|win32api.BsPushbutton|win32api.WsDisabled,
			x_offset, 0, button_width, urlbar_height,
			self.hwnd_, win32api.HMENU(IdcNavStop),
			hInstance, 0,
		); err == nil {
			self.stop_hwnd_ = stop_hwnd_
		} else {
			log.Panicln("T242: Create Button", err)
		}
		x_offset += button_width

		if edit_hwnd_, err := win32api.CreateWindowEx(
			win32api.DWORD(0),
			syscall.StringToUTF16Ptr("EDIT"), nil,
			win32api.WsChild|win32api.WsVisible|win32api.WsBorder|
				win32api.EsLeft|win32api.EsAutovscroll|win32api.EsAutohscroll|win32api.WsDisabled,
			x_offset, 0, int(rect.Right)-button_width*4, urlbar_height,
			self.hwnd_, 0,
			hInstance, 0,
		); err == nil {
			self.edit_hwnd_ = edit_hwnd_
		} else {
			log.Panicln("T242: Create Button", err)
		}
		x_offset += button_width

		// // Override the edit control's window procedure.
		self.edit_wndproc_old_ = SetWndProc(self.edit_hwnd_, EditWndProc)

		// // Associate |this| with the edit window.
		// SetUserDataPtr(edit_hwnd_, this)
		windowManager.SetRootWin(self.edit_hwnd_, self)

		rect.Top += win32api.LONG(urlbar_height)

		if !self.with_osr_ {
			if hMenu := win32api.GetMenu(self.hwnd_); hMenu != 0 {
				if hTestMenu := win32api.GetSubMenu(hMenu, 2); hTestMenu != 0 {
					if err := win32api.RemoveMenu(hTestMenu, IdTestsOsrFps, win32api.MfBycommand); err != nil {
						log.Panicln("T410:", err)
					}
					if err := win32api.RemoveMenu(hTestMenu, IdTestsOsrDsf, win32api.MfBycommand); err != nil {
						log.Panicln("T413:", err)
					}
				}
			}
		}
	} else {
		win32api.SetMenu(self.hwnd_, 0)
	}

	device_scale_factor := GetWindowScaleFactor(self.hwnd_)
	if dsfer, ok := self.browser_window_.(DeviceScaleFactorer); ok {
		dsfer.SetDeviceScaleFactor(device_scale_factor)
	}

	r := capi.CRectT{}
	r.SetX(int(rect.Left))
	r.SetY(int(rect.Top))
	r.SetWidth(int(rect.Right - rect.Left))
	r.SetHeight(int(rect.Bottom - rect.Top))

	if self.is_popup_ {
		self.browser_window_.ShowPopup(self.hwnd_, r)
	} else {
		capi.Logln("T446:", self.browser_window_, self.browser_window_.GetCClientT())
		self.browser_window_.CreateBrowser(self.initial_url, self.hwnd_, r, self.browser_settings_, nil, nil) // delegate が PDF extension を許可している)
	}
}

func (self *RootWindowWin) OnPaint() {
	var ps win32api.Paintstruct
	win32api.BeginPaint(self.hwnd_, &ps)
	win32api.EndPaint(self.hwnd_, &ps)
}

func (self *RootWindowWin) OnActivate(active bool) {
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
	if self.hwnd_ != 0 {
		if err := win32api.GetClientRect(self.hwnd_, &rect); err != nil {
			log.Panicln("T269: GetClientRect", err, self.hwnd_)
		}
	}

	if self.with_controls_ && self.edit_hwnd_ != 0 {
		button_width := GetButtonWidth(self.hwnd_)
		urlbar_height := GetURLBarHeight(self.hwnd_)
		font_height := LogicalToDevice(14, GetWindowScaleFactor(self.hwnd_))

		if font_height != self.font_height_ {
			self.font_height_ = font_height
			if self.font_ != 0 {
				win32api.DeleteObject(win32api.HGDIOBJ(self.font_))
			}

			self.font_ = win32api.CreateFont(-font_height, 0, 0, 0,
				win32api.FwDontcare, false, false, false,
				win32api.DefaultCharset,
				win32api.OutDefaultPrecis,
				win32api.ClipDefaultPrecis,
				win32api.DefaultQuality,
				win32api.DefaultPitch|win32api.FfDontcare,
				syscall.StringToUTF16Ptr("Arial"),
			)

			win32api.SendMessage(self.back_hwnd_, win32api.UINT(win32api.WmSetfont), win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.forward_hwnd_, win32api.UINT(win32api.WmSetfont), win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.reload_hwnd_, win32api.UINT(win32api.WmSetfont), win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.stop_hwnd_, win32api.UINT(win32api.WmSetfont), win32api.WPARAM(self.font_), 1)
			win32api.SendMessage(self.edit_hwnd_, win32api.UINT(win32api.WmSetfont), win32api.WPARAM(self.font_), 1)
		}
		rect.Top += win32api.LONG(urlbar_height)
		x_offset := int(rect.Left)

		var browser_hwnd win32api.HWND
		if self.browser_window_ != nil {
			browser_hwnd = GetWindowHandle(self.browser_window_.GetCBrowserT())
		}

		var hdwp win32api.HDWP
		var err error
		if browser_hwnd != 0 {
			hdwp, err = win32api.BeginDeferWindowPos(6)
		} else {
			hdwp, err = win32api.BeginDeferWindowPos(5)
		}
		if err != nil {
			log.Panicln("T317:", err)
		}

		hdwp, err = win32api.DeferWindowPos(hdwp, self.back_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32api.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.forward_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32api.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.reload_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32api.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.stop_hwnd_, 0, x_offset, 0, button_width, urlbar_height, win32api.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		x_offset += button_width
		hdwp, err = win32api.DeferWindowPos(hdwp, self.edit_hwnd_, 0, x_offset, 0, int(rect.Right)-x_offset, urlbar_height, win32api.SwpNozorder)
		if err != nil {
			log.Panicln("T322:", err)
		}

		if browser_hwnd != 0 {
			hdwp, err = win32api.DeferWindowPos(
				hdwp, browser_hwnd, 0,
				int(rect.Left), int(rect.Top),
				int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
				win32api.SwpNozorder,
			)
		}
		err = win32api.EndDeferWindowPos(hdwp)
		if err != nil {
			log.Panicln("T359:", err)
		}
	} else if self.browser_window_ != nil {
		SetBounds(self.browser_window_.GetCBrowserT(), 0, 0, uint32(rect.Right), uint32(rect.Bottom))
	}
}

func (self *RootWindowWin) OnMove() {
	browser := self.GetBrowser()
	if browser != nil {
		h := browser.GetHost()
		defer h.Unref()
		h.NotifyMoveOrResizeStarted()
	}
}

func (self *RootWindowWin) OnDpiChanged(wParam win32api.WPARAM, lParam win32api.LPARAM) {
	if win32api.LOWORD(wParam) != win32api.HIWORD(wParam) {
		log.Println("Not Implemented: Received non-square scaling factors")
		return
	}

	if self.browser_window_ != nil && self.with_osr_ {
		//	Scale factor for the new display.
		//	static_cast<float>(LOWORD(wParam)) / DPI_1X;
		display_scale_factor := float32(win32api.LOWORD(wParam)) / DPI_1X
		if dsfer, ok := self.browser_window_.(DeviceScaleFactorer); ok {
			dsfer.SetDeviceScaleFactor(display_scale_factor)
		}
	}

	rect := win32api.LParamToPRect(lParam)
	self.SetBounds(int(rect.Left), int(rect.Top),
		uint32(rect.Right-rect.Left), uint32(rect.Bottom-rect.Top),
	)
}

func (self *RootWindowWin) OnEraseBkgnd() bool {
	// Erase the background when the browser does not exist.
	return (self.GetBrowser() == nil)
}

func (self *RootWindowWin) OnClose() bool {
	if self.browser_window_ != nil && !self.browser_window_.IsClosing() {
		browser := self.GetBrowser()
		if browser != nil {
			h := browser.GetHost()
			defer h.Unref()
			h.CloseBrowser(false)
			return true
		}
	}

	return false
}

func (self *RootWindowWin) OnBrowserWindowClosing() {
	// Nothing to do
}

func (self *RootWindowWin) OnBrowserWindowDestroyed() {
	self.browser_window_ = nil
	if !self.window_destroyed_ {
		self.Close(true)
	}
	self.browser_destroyed_ = true
	self.NotifyDestroyedIfDone()
}

func (self *RootWindowWin) OnDestroyed() {
	self.window_destroyed_ = true
	self.NotifyDestroyedIfDone()
}

func (self *RootWindowWin) OnFindEvent() {
	browser := self.GetBrowser()
	host := browser.GetHost()
	defer host.Unref()
	if (self.find_state_.Flags & win32api.FrDialogterm) != 0 {
		if browser != nil {
			host.StopFinding(true)
			self.find_what_last_ = ""
			self.find_next_ = false
		}

	} else if (self.find_state_.Flags&win32api.FrFindnext) != 0 && browser != nil {
		match_case := self.find_state_.Flags&win32api.FrMatchcase != 0
		find_what := syscall.UTF16ToString(self.find_buff_[:])
		if match_case != self.find_match_case_last_ || find_what != self.find_what_last_ {
			if find_what != "" {
				host.StopFinding(true)
				self.find_next_ = false
			}
			self.find_match_case_last_ = match_case
			self.find_what_last_ = find_what
		}
		host.Find(
			0,
			find_what,
			(self.find_state_.Flags&win32api.FrDown) != 0,
			match_case, self.find_next_,
		)
		if !self.find_next_ {
			self.find_next_ = true
		}
	}
}

func (self *RootWindowWin) OnCommand(id win32api.UINT) bool {
	if id >= IdTestsFirst && id <= IdTestsLast {
		onTestCommand(self, id)
	}
	switch id {
	case IdmAbout:
		self.OnAbout()
		return true
	case IdmExit:
		windowManager.CloseAllWindows(false)
		return true
	case IdFind:
		self.OnFind()
		return true
	case IdcNavBack:
		browser := self.GetBrowser()
		if browser != nil {
			browser.GoBack()
		}
		return true
	case IdcNavForward:
		browser := self.GetBrowser()
		if browser != nil {
			browser.GoForward()
		}
		return true
	case IdcNavReload:
		browser := self.GetBrowser()
		if browser != nil {
			browser.Reload()
		}
		return true
	case IdcNavStop:
		browser := self.GetBrowser()
		if browser != nil {
			browser.StopLoad()
		}
		return true
	}
	return false
}

func (self *RootWindowWin) OnAbout() {
	hInstance, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T594:", err)
	}
	win32api.DialogBoxParam(
		win32api.HINSTANCE(hInstance),
		win32api.MakeIntResource(IddAboutbox),
		self.hwnd_,
		win32api.DLGPROC(syscall.NewCallback(AboutWndProc)),
		0,
	)
}

func AboutWndProc(hDlg win32api.HWND, message win32api.MessageId, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	switch message {
	case win32api.WmInitdialog:
		return win32api.True
	case win32api.WmCommand:
		action := int(win32api.LOWORD(wParam))
		if action == win32api.Idok || action == win32api.Idcancel {
			win32api.EndDialog(hDlg, win32api.INT_PTR(action))
			return win32api.True
		}
	}
	return win32api.False
}

func (rw *RootWindowWin) OnFind() {
	if rw.find_hwnd_ != 0 {
		win32api.SetFocus(rw.find_hwnd_)
		return
	}
	rw.find_state_ = win32api.Findreplace{}
	rw.find_state_.StructSize = win32api.DWORD((unsafe.Sizeof(win32api.Findreplace{})))
	rw.find_state_.Owner = rw.hwnd_
	rw.find_state_.FindWhat = (*uint16)(unsafe.Pointer(&rw.find_buff_))
	rw.find_state_.FindWhatLen = win32api.WORD(unsafe.Sizeof(rw.find_buff_))
	rw.find_state_.Flags = win32api.FrHidewholeword | win32api.FrDown

	rw.find_hwnd_ = win32api.FindText(&rw.find_state_)
	if rw.find_hwnd_ == 0 {
		r := win32api.CommDlgExtendedError()
		log.Panicf("T647: %x\n", r)
	}

	rw.find_wndproc_old_ = SetWndProc(rw.find_hwnd_, FindWndProc)
	windowManager.SetRootWin(rw.find_hwnd_, rw)
}

func FindWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	self, ok := windowManager.GetRootWin(hWnd)
	if !ok {
		log.Panicln("T656:", hWnd)
	}
	if hWnd != self.find_hwnd_ {
		log.Panicln("T659: find_hwnd_ not match", hWnd, self.find_hwnd_)
	}

	msgId := win32api.MessageId(message)
	switch msgId {
	case win32api.WmActivate:
		// nothing to do on single thread message loop
		return 0
	case win32api.WmNcdestroy:
		windowManager.RemoveRootWin(hWnd)
		self.find_hwnd_ = 0
	}

	return win32api.CallWindowProc(self.find_wndproc_old_, hWnd, message, wParam, lParam)
}

func onTestCommand(rw *RootWindowWin, id win32api.UINT) {
	browser := rw.GetBrowser()
	if browser == nil {
		return
	}
	switch id {
	case IdTestsGetsource:
		runGetSourceTest(rw.browser_window_)

	case IdTestsGettext:
		runGetTextTest(rw.browser_window_)

	case IdTestsWindowNew:
		runNewWindowTest(rw.initial_url, rw.browser_window_)

	case IdTestsWindowPopup:
		runPopupWindowTest(rw.browser_window_)

	case IdTestsRequest:
		runRequestTest(rw.browser_window_)

	case IdTestsPluginInfo:
		runPluginInfo(rw.browser_window_)

	case IdTestsZoomIn:
		ModifyZoom(rw.browser_window_.GetCBrowserT(), 0.5)

	case IdTestsZoomOut:
		ModifyZoom(rw.browser_window_.GetCBrowserT(), -0.5)

	case IdTestsZoomReset:
		rw.browser_window_.GetCBrowserT().GetHost().SetZoomLevel(0.0)

	case IdTestsOsrFps:
		PromptFPS(rw.browser_window_.GetCBrowserT())
	case IdTestsOsrDsf:
		PromptDSF(rw.browser_window_.GetCBrowserT())
	case IdTestsTracingBegin:
		BeginTracing()
	case IdTestsTracingEnd:
		EndTracing(rw.browser_window_.GetCBrowserT())
	case IdTestsPrint:
		h := browser.GetHost()
		defer h.Unref()
		h.Print()
	case IdTestsPrintToPdf:
		PrintToPdf(rw.browser_window_.GetCBrowserT())
	case IdTestsMuteAudio:
		MuteAudio(rw.browser_window_.GetCBrowserT(), true)
	case IdTestsUnmuteAudio:
		MuteAudio(rw.browser_window_.GetCBrowserT(), false)
	case IdTestsOtherTests:
		RunOtherTests(rw.browser_window_.GetCBrowserT())
	}
}

func runGetSourceTest(browser BrowserWindow) {
	GetSource(browser)
}

func runGetTextTest(browser BrowserWindow) {
	GetText(browser)
}

func runNewWindowTest(initial_url string, browser BrowserWindow) {
	browserSettings := capi.NewCBrowserSettingsT()
	rect := win32api.Rect{}
	h := browser.GetCBrowserT().GetHost()
	defer h.Unref()
	with_osr := h.IsWindowRenderingDisabled()

	config := mainConfig
	config.main_url = initial_url
	config.use_windowless_rendering = with_osr
	windowManager.CreateRootWindow(config, false, rect, browserSettings)
}

func runPopupWindowTest(browser BrowserWindow) {
	browser.GetCBrowserT().GetMainFrame().ExecuteJavaScript(
		"window.open('http://www.google.com');", "about:blank", 0)
}

func runRequestTest(browser BrowserWindow) {
	frame := browser.GetCBrowserT().GetMainFrame()
	defer frame.Unref()

	url := frame.GetUrl()
	if !strings.HasPrefix(url, kTestOrigin) {
		msg := "Please first navigate to a http://tests/ URL. " +
			"For example, first load Tests > Other Tests."
		frame.ExecuteJavaScript("alert('"+msg+"');", url, 0)
		return
	}

	request := capi.RequestCreate()
	request.SetUrl(kTestOrigin + kTestRequestPage)

	// Add post data to the request.  The correct method and content-
	// type headers will be set by CEF.
	postDataElement := capi.PostDataElementCreate()
	postDataElement.SetToBytes([]byte("arg1=val1&arg2=val2"))
	postData := capi.PostDataCreate()
	postData.AddElement(postDataElement)
	request.SetPostData(postData)

	// Add a custom header
	h := cef.NewStringMultimap()
	capi.StringMultimapAppend(h.CefObject(), "X-My-Header", "My Header Value")
	request.SetHeaderMap(h.CefObject())

	frame.LoadRequest(request)
}

func runPluginInfo(browser BrowserWindow) {
	visitor := GetPlugInInfoVisitor(browser.GetCBrowserT(), browser.GetResourceManager())
	capi.VisitWebPluginInfo(visitor)
}

func PromptFPS(browser *capi.CBrowserT) {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			PromptFPS(browser)
		}))
		return
	}
	fps := browser.GetHost().GetWindowlessFrameRate()
	Prompt(browser, kPromptFPS, "Enter FPS", strconv.Itoa(fps))
}

func Prompt(browser *capi.CBrowserT, prompt string, label string, default_value string) {

	// Prompt the user for a new value. Works as follows:
	// 1. Show a prompt() dialog via JavaScript.
	// 2. Pass the result to window.cefQuery().
	// 3. Handle the result in PromptHandler::OnQuery.
	code := fmt.Sprintf("window.%s({'request': '%s' + prompt('%s', '%s')});", jsQueryFunctionName,
		prompt, label, default_value)
	browser.GetMainFrame().ExecuteJavaScript(
		code, browser.GetMainFrame().GetUrl(), 0)
}

func PromptDSF(browser *capi.CBrowserT) {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			PromptDSF(browser)
		}))
		return
	}
	dsf := windowManager.GetRootWinForBrowser(browser.GetIdentifier()).GetDeviceScaleFactor()
	Prompt(browser, kPromptDSF, "Enter DSF", strconv.FormatFloat(float64(dsf), 'f', -1, 32))
}

func ModifyZoom(browser *capi.CBrowserT, delta float64) {
	browser.GetHost().SetZoomLevel(browser.GetHost().GetZoomLevel() + delta)
}

func (self *RootWindowWin) NotifyDestroyedIfDone() {
	if self.window_destroyed_ && self.browser_destroyed_ {
		windowManager.OnRootWindowDestroyed(self)
	}
}

func (self *RootWindowWin) SetBounds(x, y int, width, height uint32) {
	if self.hwnd_ != 0 {
		win32api.SetWindowPos(self.hwnd_, 0, x, y, int(width), int(height), win32api.SwpNozorder)
	}
}

func (self *RootWindowWin) GetBrowser() *capi.CBrowserT {
	return self.browser_window_.GetCBrowserT()
}

const MaxUrlLength = 255

func EditWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	self, ok := windowManager.GetRootWin(hWnd)
	if !ok {
		log.Panicln("T386:", hWnd)
	}
	if hWnd != self.edit_hwnd_ {
		log.Panicln("T391: edit_hwnd_ not match", hWnd, self.edit_hwnd_)
	}

	msgId := win32api.MessageId(message)
	switch msgId {
	case win32api.WmChar:
		if wParam == win32api.VkReturn {
			browser := self.GetBrowser()
			urlstr := [MaxUrlLength + 1]uint16{}
			urlstr[0] = MaxUrlLength
			sp := win32api.LPARAM(uintptr(unsafe.Pointer(&urlstr)))
			result := win32api.SendMessage(hWnd, win32api.EmGetline, 0, sp)
			if result > 0 {
				runes := utf16.Decode(urlstr[0:result])
				url := string(runes)
				browser.GetMainFrame().LoadUrl(url)
			}
			return 0
		}
	case win32api.WmNcdestroy:
		windowManager.RemoveRootWin(hWnd)
		self.edit_hwnd_ = 0
	}

	return win32api.CallWindowProc(self.edit_wndproc_old_, hWnd, message, wParam, lParam)
}

func (self *RootWindowWin) Close(force bool) {
	if self.hwnd_ != 0 {
		if force {
			win32api.DestroyWindow(self.hwnd_)
		} else {
			win32api.PostMessage(self.hwnd_, win32api.UINT(win32api.WmClose), 0, 0)
		}
	}
}

func (self *RootWindowWin) OnBrowserCreated(browser *capi.CBrowserT) {
	if self.is_popup_ {
		// For popup browsers create the root window once the browser has been
		// created.
		self.CreateWindow()
	} else {
		// Make sure the browser is sized correctly.
		self.OnSize(false)
	}
	windowManager.OnBrowserCreated(self, browser)
}

func (rw *RootWindowWin) OnLoadingStateChange(
	isLoading bool,
	canGoBack bool,
	canGoForward bool,
) {
	win32api.EnableWindow(rw.back_hwnd_, canGoBack)
	win32api.EnableWindow(rw.forward_hwnd_, canGoForward)
	win32api.EnableWindow(rw.reload_hwnd_, !isLoading)
	win32api.EnableWindow(rw.stop_hwnd_, isLoading)
	win32api.EnableWindow(rw.edit_hwnd_, true)
}

func (rw *RootWindowWin) GetDeviceScaleFactor() float32 {
	if rw.browser_window_ != nil && rw.with_osr_ {
		if dsfer, ok := rw.browser_window_.(DeviceScaleFactorer); ok {
			return dsfer.GetDeviceScaleFactor()
		}
	}
	log.Panicln("T1003: Not Reacehd")
	return 0
}

func BeginTracing() {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			BeginTracing()
		}))
		return
	}
	capi.BeginTracing("", nil)
}

type endTraceCallback struct {
	endTracingCallback    *capi.CEndTracingCallbackT
	browser               *capi.CBrowserT
	runFileDialogCallback *capi.CRunFileDialogCallbackT
}

func init() {
	var etc *endTraceCallback

	// capi.CEndTracingCallbackT
	var _ capi.OnEndTracingCompleteHandler = etc

	// capi.CRunFileDialogCallbackT
	var _ capi.OnFileDialogDismissedHandler = etc
}

func (etc *endTraceCallback) OnEndTracingComplete(
	self *capi.CEndTracingCallbackT,
	tracing_file string,
) {
	frame := etc.browser.GetMainFrame()
	defer frame.Unref()

	etc.browser.Unref() // unref before Popup alert dialog box 
	etc.endTracingCallback.Unref() // .UnbindAll()

	url := frame.GetUrl()
	frame.ExecuteJavaScript(fmt.Sprintf("alert('File \"%s\" saved successfully');", tracing_file), url, 0)
}

func EndTracing(browser *capi.CBrowserT) {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			EndTracing(browser)
		}))
		return
	}
	etc := &endTraceCallback{}
	etc.browser = browser.NewRef()

	etc.endTracingCallback = capi.NewCEndTracingCallbackT(etc)
	callback := capi.NewCRunFileDialogCallbackT(etc)
	defer callback.Unref()
	path := GetDownloadPath("trace.txt")
	accept_filters := cef.NewStringList()
	browser.GetHost().RunFileDialog(
		capi.FileDialogSave|capi.FileDialogOverwritepromptFlag,
		"", // title
		path,
		accept_filters.CefObject(),
		0,
		callback,
	)
}

func (etc *endTraceCallback) OnFileDialogDismissed(
	self *capi.CRunFileDialogCallbackT,
	selected_accept_filter int,
	file_paths capi.CStringListT,
) {
	// etc.UnrefCEndTracingCallbackT()
	defer etc.runFileDialogCallback.Unref() // .UnbindAll()

	cb := etc.endTracingCallback
	if capi.StringListSize(file_paths) > 0 {
		if ok, file := capi.StringListValue(file_paths, 0); ok {
			capi.EndTracing(file, cb)
			return
		}
	}
	capi.EndTracing("", cb)
}

type printPdfCallback struct {
	browser          *capi.CBrowserT
	pdfPrintCallback *capi.CPdfPrintCallbackT
}

func init() {
	var ppc *printPdfCallback

	// capi.CRunFileDialogCallbackT
	var _ capi.OnFileDialogDismissedHandler = ppc

	// capi.CPdfPrintCallbackT
	var _ capi.OnPdfPrintFinishedHandler = ppc
}

func (ppc *printPdfCallback) OnPdfPrintFinished(
	self *capi.CPdfPrintCallbackT,
	path string,
	ok bool,
) {
	defer func() {
		ppc.browser.Unref()
		ppc.pdfPrintCallback.Unref() // UnbindAll()
	}()

	frame := ppc.browser.GetMainFrame()
	defer frame.Unref()

	url := frame.GetUrl()
	var alertStmt string
	if ok {
		alertStmt = fmt.Sprintf("alert('File \"%s\" saved successfully');", path)
	} else {
		alertStmt = fmt.Sprintf("alert('File \"%s\" failed to save');", path)
	}
	frame.ExecuteJavaScript(alertStmt, url, 0)
}

func (ppc *printPdfCallback) OnFileDialogDismissed(
	self *capi.CRunFileDialogCallbackT,
	selected_accept_filter int,
	file_paths capi.CStringListT,
) {
	if capi.StringListSize(file_paths) > 0 {
		settings := capi.NewCPdfPrintSettingsT()
		settings.SetHeaderFooterEnabled(true)

		frame := ppc.browser.GetMainFrame()
		defer frame.Unref()

		settings.SetHeaderFooterUrl(frame.GetUrl())
		defer ppc.pdfPrintCallback.Unref()
		cb := ppc.pdfPrintCallback
		if ok, file := capi.StringListValue(file_paths, 0); ok {
			h := ppc.browser.GetHost()
			defer h.Unref()
			h.PrintToPdf(file, settings, cb)
		}
	}
}

func PrintToPdf(browser *capi.CBrowserT) {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			PrintToPdf(browser)
		}))
		return
	}
	ppc := &printPdfCallback{}
	ppc.browser = browser.NewRef()
	ppc.pdfPrintCallback = capi.NewCPdfPrintCallbackT(ppc)
	callback := capi.NewCRunFileDialogCallbackT(ppc)
	defer callback.Unref()
	accept_filters := cef.NewStringList()
	accept_filters.Append(".pdf")
	path := GetDownloadPath("output.pdf")
	log.Println("T1153:", path)

	h := browser.GetHost()
	defer h.Unref()
	h.RunFileDialog(
		capi.FileDialogSave|capi.FileDialogOverwritepromptFlag,
		"", // title
		path,
		accept_filters.CefObject(),
		0,
		callback,
	)
}

func MuteAudio(browser *capi.CBrowserT, mute bool) {
	host := browser.GetHost()
	defer host.Unref()
	host.SetAudioMuted(mute)
}

func RunOtherTests(browser *capi.CBrowserT) {
	browser.GetMainFrame().LoadUrl("http://tests/other_tests")
}
