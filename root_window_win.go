package main

import (
	"log"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type RootWindowWin struct {
	with_controls_    bool
	always_on_top_    bool
	no_activate_      bool
	is_popup_         bool
	start_rect_       win32api.Rect
	browser_settings_ *capi.CBrowserSettingsT
	browser_window_   *BrowserWindow
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

	find_hwnd_        win32api.HWND
	find_message_id_  win32api.UINT
	find_wndproc_old_ win32api.WNDPROC
	find_state_       win32api.Findreplace
	find_buff_        [80]uint16
	find_what_last_   string
	find_next_        bool
	find_match_case_last_ bool

	called_enable_non_client_dpi_scaling_ bool

	window_destroyed_  bool
	browser_destroyed_ bool
}

func (rw *RootWindowWin) Init(
	is_popup bool,
	with_controls bool,
	rect win32api.Rect,
	always_on_top bool,
	
	no_activate bool,
	settings *capi.CBrowserSettingsT,
) *BrowserWindow {
	rw.start_rect_ = rect
	rw.always_on_top_ = always_on_top
	rw.no_activate_ = no_activate

	rw.draggable_region_ = win32api.CreateRectRgn(0, 0, 0, 0)
	rw.with_controls_ = with_controls
	rw.is_popup_ = is_popup
	rw.browser_settings_ = settings
	rw.browser_window_ = NewBrowserWindow(rw)

	return rw.browser_window_
}

func (rw *RootWindowWin) CreateWindow(
	key int,
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
	r, err := win32api.RegisterWindowMessage(syscall.StringToUTF16Ptr(win32const.Findmsgstring))
	if err != nil {
		log.Panicln("T93:", err)
	}
	rw.find_message_id_ = r

	dwStyle := win32api.DWORD(win32const.WsOverlappedwindow | win32const.WsClipchildren)

	dwExStyle := win32api.DWORD(0)
	if rw.always_on_top_ {
		dwExStyle |= win32const.WsExTopmost
	}
	if rw.no_activate_ {
		dwExStyle |= win32const.WsExNoactivate
	}

	var x, y, width, height int
	if win32api.IsRectEmpty(&rw.start_rect_) {
		x = win32const.CwUsedefault
		y = win32const.CwUsedefault
		width = win32const.CwUsedefault
		height = win32const.CwUsedefault
	} else {
		if _, err := win32api.AdjustWindowRectEx(&rw.start_rect_, dwStyle, true, dwExStyle); err != nil {
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
		win32api.LPVOID(key),
	)
	if wnd == 0 || err != nil || wnd != rw.hwnd_ {
		log.Panicln("T52: Failed to CreateWindowsEx", wnd, err, rw.hwnd_)
	}

	win32api.ShowWindow(rw.hwnd_, win32const.SwShownormal)
	if !win32api.UpdateWindow(rw.hwnd_) {
		log.Panicln("T63: ShowWindow")
	}
}

func RootWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	var self *RootWindowWin
	if message != win32const.WmNccreate {
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

	switch message {
	case win32const.WmCommand:
		if self.OnCommand(win32api.UINT(win32api.LOWORD(wParam))) {
			return 0
		}

	case win32const.WmGetobject:
		// Only the lower 32 bits of lParam are valid when checking the object id
		// because it sometimes gets sign-extended incorrectly (but not always).
		obj_id := win32api.DWORD(lParam)

		// Accessibility readers will send an OBJID_CLIENT message.
		if win32api.DWORD(0xffffffff&win32const.ObjidClient) == obj_id {
			if self.GetBrowser() != nil && self.GetBrowser().GetHost() != nil {
				self.GetBrowser().GetHost().SetAccessibilityState(capi.StateEnabled)
			}
		}

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

	case win32const.WmMoving, win32const.WmMove:
		self.OnMove()
		return 0

	case win32const.WmDpichanged:
		self.OnDpiChanged(wParam, lParam)

	case win32const.WmErasebkgnd:
		if !self.OnEraseBkgnd() {
			return 0 // Don't erase the background.
		}

	case win32const.WmEntermenuloop:
		if wParam == 0 {
			// Entering the menu loop for the application menu.
			capi.SetOsmodalLoop(true)
		}

	case win32const.WmExitmenuloop:
		if wParam == 0 {
			// Exiting the menu loop for the application menu.
			capi.SetOsmodalLoop(false)
		}

	case win32const.WmClose:
		if self.OnClose() {
			return 0
		}

	case win32const.WmNchittest:
		hit := win32api.DefWindowProc(hWnd, message, wParam, lParam)
		if hit == win32const.Htclient {
			points := win32api.Makepoints(lParam)
			point := win32api.Point{X: win32api.LONG(points.X), Y: win32api.LONG(points.Y)}
			win32api.ScreenToClient(hWnd, &point)
			if win32api.PtInRegion(self.draggable_region_, int(point.X), int(point.Y)) {
				// If cursor is inside a draggable region return HTCAPTION to allow
				// dragging.
				return win32const.Htcaption
			}
		}
		return hit

	case win32const.WmNccreate:
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

	case win32const.WmCreate:
		cs := win32api.ToPCreatestruct(lParam)
		self.OnCreate(cs)

	case win32const.WmNcdestroy:
		// win32api.SetUserDataPtr(wWnd, nil)
		windowManager.RemoveRootWin(hWnd)
		self.hwnd_ = 0
		self.OnDestroyed()
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
			self.hwnd_, win32api.HMENU(IdcNavBack),
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
			self.hwnd_, win32api.HMENU(IdcNavForward),
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
			self.hwnd_, win32api.HMENU(IdcNavReload),
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
			self.hwnd_, win32api.HMENU(IdcNavStop),
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
		windowManager.SetRootWin(edit_hwnd_, self)

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
	if self.is_popup_ {
		bwHwnd := self.browser_window_.GetWindowHandle()
		if bwHwnd != 0 {
			if _, err := win32api.SetParent(bwHwnd, self.hwnd_); err != nil {
				log.Panicln("T368:", err)
			}
			if _, err := win32api.SetWindowPos(bwHwnd, 0,
				int(rect.Left), int(rect.Top), int(rect.Right-rect.Left), int(rect.Bottom-rect.Top),
				win32const.SwpNozorder|win32const.SwpNoactivate); err != nil {
				log.Panicln("T372:", err)
			}
			if no_activate, err := win32api.GetWindowLongPtr(self.hwnd_, win32const.GwlExstyle); err == nil {
				swFlag := win32const.SwShow
				if no_activate&win32const.WsExNoactivate != 0 {
					swFlag = win32const.SwShownoactivate
				}
				win32api.ShowWindow(bwHwnd, swFlag)
			} else {
				log.Panicln("T372:", err)
			}
		}
	} else {
		// parentHwnd := capi.CWindowHandleT(unsafe.Pointer(uintptr(self.hwnd_)))
		parentHwnd := capi.ToCWindowHandleT(syscall.Handle(self.hwnd_))
		self.browser_window_.CreateBrowser(parentHwnd, r, self.browser_settings_, nil, nil) // delegate が PDF extension を許可している)
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

func (self *RootWindowWin) OnMove() {
	browser := self.GetBrowser()
	if browser != nil {
		browser.GetHost().NotifyMoveOrResizeStarted()
	}
}

func (self *RootWindowWin) OnDpiChanged(wParam win32api.WPARAM, lParam win32api.LPARAM) {
	if win32api.LOWORD(wParam) != win32api.HIWORD(wParam) {
		log.Println("Not Implemented: Received non-square scaling factors")
		return
	}

	// if self.browser_window_ != 0 && with_osr_ {
	//	Scale factor for the new display.
	//	const float display_scale_factor =
	//	static_cast<float>(LOWORD(wParam)) / DPI_1X;
	//	browser_window_->SetDeviceScaleFactor(display_scale_factor);
	// }

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
			browser.GetHost().CloseBrowser(false)
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

	if (self.find_state_.Flags & win32const.FrDialogterm) != 0 {
		if browser != nil {
			browser.GetHost().StopFinding(true)
			self.find_what_last_ = ""
			self.find_next_ = false
		}

	} else if ((self.find_state_.Flags & win32const.FrFindnext) != 0 && browser != nil) {
		match_case := self.find_state_.Flags & win32const.FrMatchcase != 0
		find_what := syscall.UTF16ToString(self.find_buff_[:])
		if (match_case != self.find_match_case_last_ || find_what != self.find_what_last_) {
			if find_what != "" {
				browser.GetHost().StopFinding(true)
				self.find_next_ = false
			}
			self.find_match_case_last_ = match_case
			self.find_what_last_ = find_what
		}
		browser.GetHost().Find(
			0,
			find_what,
			(self.find_state_.Flags & win32const.FrDown) != 0,
			 match_case, self.find_next_,
		)
		if (!self.find_next_) {
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
		win32api.HINSTANCE(syscall.Handle(hInstance)),
		win32api.MakeIntResource(IddAboutbox),
		self.hwnd_,
		win32api.DLGPROC(syscall.NewCallback(AboutWndProc)),
		0,
	)
}

func AboutWndProc(hDlg win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	switch message {
	case win32const.WmInitdialog:
		return win32const.True
	case win32const.WmCommand:
		action := int(win32api.LOWORD(wParam))
		if action == win32const.Idok || action == win32const.Idcancel {
			win32api.EndDialog(hDlg, win32api.INT_PTR(action))
			return win32const.True
		}
	}
	return win32const.False
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
	rw.find_state_.Flags = win32const.FrHidewholeword | win32const.FrDown

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

	switch message {
	case win32const.WmActivate:
		// nothing to do on single thread message loop
		return 0
	case win32const.WmNcdestroy:
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
	}
}

type myStringVisitor struct {
	browserWindow *BrowserWindow
}

func (sv *myStringVisitor) Visit(self *capi.CStringVisitorT, cstring string) {
	s := strings.Replace(cstring, ">", "&gt;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	ss := "<html><meta charset=\"utf-8\"><body bgcolor=\"white\">Source:<pre>" + s + "</pre></body></html>"
	log.Println("T761:", ss)
	sv.browserWindow.resorceManager.text = []byte(ss)
	sv.browserWindow.resorceManager.mime = "text/html"
	sv.browserWindow.browser_.GetMainFrame().LoadUrl(kTestOrigin + kTestGetSourcePage)
}

func runGetSourceTest(browser *BrowserWindow) {
	mySv := myStringVisitor{
		browserWindow: browser,
	}
	sv := capi.AllocCStringVisitorT()
	sv.Bind(&mySv)
	browser.browser_.GetMainFrame().GetSource(sv)
}

func (self *RootWindowWin) NotifyDestroyedIfDone() {
	if self.window_destroyed_ && self.browser_destroyed_ {
		OnRootWindowDestroyed(self)
	}
}

func (self *RootWindowWin) SetBounds(x, y int, width, height uint32) {
	if self.hwnd_ != 0 {
		win32api.SetWindowPos(self.hwnd_, 0, x, y, int(width), int(height), win32const.SwpNozorder)
	}
}

func (self *RootWindowWin) GetBrowser() *capi.CBrowserT {
	return self.browser_window_.browser_
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
			win32api.PostMessage(self.hwnd_, win32const.WmClose, 0, 0)
		}
	}
}

func (self *RootWindowWin) OnBrowserCreated(browser *capi.CBrowserT) {
	self.OnSize(false)
}
