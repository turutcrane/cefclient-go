package main

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type BrowserWindowOsr struct {
	rootWin_             *RootWindowWin
	browser_             *capi.CBrowserT
	is_closing_          bool
	hidden_              bool
	device_scale_factor_ float32
	renderer_            *OsrRenderer
	osr_hwnd_            win32api.HWND
	hdc_                 win32api.HDC
	hrc_                 win32api.HGLRC
	painting_popup_      bool
	background_color     capi.CColorT

	client_rect_ capi.CRectT

	resourceManager ResourceManager

	// Mouse state tracking osr_window_win.h
	last_click_x_            int
	last_click_y_            int
	last_click_button_       capi.CMouseButtonTypeT
	last_click_time_         win32api.LONG
	last_click_count_        int
	last_mouse_pos_          win32api.Point
	current_mouse_pos_       win32api.Point
	mouse_rotation_          bool
	mouse_tracking_          bool
	last_mouse_down_on_view_ bool

	external_begin_frame_enabled bool
	windowless_frame_rate        int
	begin_frame_pending_         bool

	capi.RefToCClientT
	capi.RefToCLifeSpanHandlerT
	capi.RefToCLoadHandlerT
	capi.RefToCRequestHandlerT
	capi.RefToCDisplayHandlerT
	capi.RefToCRenderHandlerT
}

type nullCClientT struct{}

func (*nullCClientT) GetRenderHandler(self *capi.CClientT) (ret *capi.CRenderHandlerT) {
	rh := capi.AllocCRenderHandlerT() // has no hander routine
	return rh
}

func init() {
	var bwo *BrowserWindowOsr
	// capi.CClientT
	var _ capi.GetLifeSpanHandlerHandler = bwo
	var _ capi.CClientTGetLoadHandlerHandler = bwo
	var _ capi.GetRequestHandlerHandler = bwo
	var _ capi.GetDisplayHandlerHandler = bwo
	var _ capi.GetRenderHandlerHandler = bwo
	var _ capi.GetRenderHandlerHandler = (*nullCClientT)(nil)

	// capi.CLifeSpanHandlerT
	var _ capi.DoCloseHandler = bwo
	var _ capi.OnAfterCreatedHandler = bwo
	var _ capi.OnBeforeCloseHandler = bwo
	var _ capi.OnBeforePopupHandler = bwo

	// capi.CLoadHandlerT
	var _ capi.OnLoadingStateChangeHandler = bwo

	// capi.CRequestHandlerT
	var _ capi.CRequestHandlerTGetResourceRequestHandlerHandler = bwo
	// var _ capi.OnOpenUrlfromTabHandler = (*BrowserWindowOsr)(nil)

	// capi.CDisplayHandlerT
	var _ capi.OnAddressChangeHandler = bwo

	// capi.CRenderHandlerT
	var _ capi.GetRootScreenRectHandler = bwo
	var _ capi.GetViewRectHandler = bwo
	var _ capi.GetScreenPointHandler = bwo
	var _ capi.GetScreenInfoHandler = bwo
	var _ capi.OnPaintHandler = bwo
}

func NewBrowserWindowOsr(
	rootWindow *RootWindowWin,
	show_update_rect bool,
	external_begin_frame_enabled bool,
	windowless_frame_rate int,
	background_color capi.CColorT,
) *BrowserWindowOsr {
	bwo := &BrowserWindowOsr{}
	bwo.rootWin_ = rootWindow
	bwo.resourceManager.rh = map[string]*capi.CResourceHandlerT{}

	bwo.external_begin_frame_enabled = external_begin_frame_enabled
	bwo.windowless_frame_rate = windowless_frame_rate
	bwo.background_color = background_color

	bwo.renderer_ = NewOsrRenderer(show_update_rect, bwo.background_color)

	capi.AllocCLifeSpanHandlerT().Bind(bwo)
	capi.AllocCClientT().Bind(bwo)
	capi.AllocCLoadHandlerT().Bind(bwo)

	capi.AllocCRequestHandlerT().Bind(bwo)
	capi.AllocCResourceRequestHandlerT().Bind(&bwo.resourceManager)

	capi.AllocCDisplayHandlerT().Bind(bwo)
	capi.AllocCResponseFilterT().Bind(bwo)

	capi.AllocCRenderHandlerT().Bind(bwo)

	return bwo
}

func (bwo *BrowserWindowOsr) CreateBrowser(
	initial_url string,
	parentHwnd win32api.HWND,
	rect capi.CRectT,
	settings *capi.CBrowserSettingsT,
	extra_info *capi.CDictionaryValueT,
	request_context *capi.CRequestContextT,
) {
	if !capi.CurrentlyOn(capi.TidUi) {
		log.Panicln("T132: Not on TidUi")
	}

	bwo.Create(parentHwnd, rect)

	windowInfo := &capi.CWindowInfoT{}
	windowInfo.SetWindowlessRenderingEnabled(true)
	windowInfo.SetParentWindow(capi.ToCWindowHandleT(syscall.Handle(bwo.osr_hwnd_)))
	windowInfo.SetExternalBeginFrameEnabled(bwo.external_begin_frame_enabled)
	if exStyle, err := win32api.GetWindowLongPtr(parentHwnd, win32const.GwlExstyle); err == nil {
		if exStyle&win32const.WsExNoactivate != 0 {
			windowInfo.SetExStyle(windowInfo.ExStyle() | win32const.WsExNoactivate)
		}
	}
	capi.BrowserHostCreateBrowser(
		windowInfo,
		bwo.GetCClientT(),
		initial_url,
		settings,
		extra_info,
		request_context,
	)
}

func (bwo *BrowserWindowOsr) Create(
	parent_hwnd win32api.HWND,
	rect capi.CRectT,
) {
	window_class := "Client_OsrWindow"

	hInst, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T160:", err)
	}
	background_brush := win32api.CreateSolidBrush(
		RGB(capi.ColorGetR(bwo.background_color),
			capi.ColorGetG(bwo.background_color),
			capi.ColorGetB(bwo.background_color)),
	)

	// |browser_background_color_| should remain 0 to enable transparent painting.
	// if (!use_transparent_painting) {
	// 	browser_background_color_ = background_color_;
	// }
	RegisterOsrClass(win32api.HINSTANCE(hInst), window_class, background_brush)
	if err != nil {
		log.Panicln("T93:", err)
	}

	dwExStyle := win32api.DWORD(0)
	if exStyle, err := win32api.GetWindowLongPtr(parent_hwnd, win32const.GwlExstyle); err == nil {
		if exStyle&win32const.WsExNoactivate != 0 {
			dwExStyle |= win32const.WsExNoactivate
		}
	}

	bwo.osr_hwnd_, err = win32api.CreateWindowEx(
		dwExStyle,
		syscall.StringToUTF16Ptr(window_class),
		nil,
		win32const.WsBorder|win32const.WsChild|win32const.WsClipchildren|win32const.WsVisible,
		rect.X(), rect.Y(), rect.Width(), rect.Height(),
		parent_hwnd,
		0,
		win32api.HINSTANCE(hInst),
		0,
	)
	if err != nil {
		log.Panicln("124: Failed to CreateWindowsEx", err)
	}
	windowManager.SetBrowserWindowOsr(bwo.osr_hwnd_, bwo)

	bwo.client_rect_ = rect

	// ime_handler_.reset(new OsrImeHandlerWin(hwnd_));
}

func (bwo *BrowserWindowOsr) GetCBrowserT() *capi.CBrowserT {
	return bwo.browser_
}

func (bwo *BrowserWindowOsr) GetResourceManager() *ResourceManager {
	return &bwo.resourceManager
}

func (bw *BrowserWindowOsr) OnLoadingStateChange(
	self *capi.CLoadHandlerT,
	browser *capi.CBrowserT,
	isLoading bool,
	canGoBack bool,
	canGoForward bool,
) {
	rootWin := bw.rootWin_

	rootWin.OnLoadingStateChange(isLoading, canGoBack, canGoForward)
}

func (bwo *BrowserWindowOsr) GetLifeSpanHandler(*capi.CClientT) *capi.CLifeSpanHandlerT {
	return bwo.GetCLifeSpanHandlerT()
}

func (bwo *BrowserWindowOsr) GetLoadHandler(*capi.CClientT) *capi.CLoadHandlerT {
	return bwo.GetCLoadHandlerT()
}

func (bwo *BrowserWindowOsr) GetRequestHandler(*capi.CClientT) *capi.CRequestHandlerT {
	return bwo.GetCRequestHandlerT()
}

func (bwo *BrowserWindowOsr) GetDisplayHandler(*capi.CClientT) *capi.CDisplayHandlerT {
	return bwo.GetCDisplayHandlerT()
}

func (bwo *BrowserWindowOsr) GetRenderHandler(*capi.CClientT) *capi.CRenderHandlerT {
	handler := bwo.GetCRenderHandlerT()
	return handler
}

func (bwo *BrowserWindowOsr) OnBeforeClose(self *capi.CLifeSpanHandlerT, browser *capi.CBrowserT) {

	// render_handler_->SetBrowser(nullptr);
	// render_handler_.reset();
	bwo.browser_ = nil

	// Destroy the native window.
	win32api.DestroyWindow(bwo.osr_hwnd_)

	// ime_handler_.reset();
	bwo.osr_hwnd_ = 0

	bwo.rootWin_.OnBrowserWindowDestroyed()
	var nullClient *nullCClientT

	bwo.GetCClientT().UnbindAll()
	bwo.GetCClientT().Bind(nullClient) // nullClient returns dummy render handler

	bwo.GetCLifeSpanHandlerT().UnbindAll()
	bwo.GetCLoadHandlerT().UnbindAll()
	bwo.GetCRequestHandlerT().UnbindAll()
	bwo.GetCDisplayHandlerT().UnbindAll()
	bwo.GetCRenderHandlerT().UnbindAll()
	bwo.resourceManager.GetCResourceRequestHandlerT().UnbindAll()

}

func (bwo *BrowserWindowOsr) OnAfterCreated(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) {
	if bwo.browser_ == nil {
		bwo.browser_ = browser
	} else {
		log.Println("T99:", "OnAfterCreated, Not set bwo.browser_")
	}

	// if (hwnd_) {
	// // The native window will already exist for non-popup browsers.
	// EnsureRenderHandler();
	// render_handler_->SetBrowser(browser);
	// }
	if bwo.osr_hwnd_ != 0 {
		if bwo.browser_ != nil && bwo.external_begin_frame_enabled {
			// Start the BeginFrame timer.
			bwo.Invalidate()
		}
	}

	if bwo.osr_hwnd_ != 0 {
		// Show the browser window. Called asynchronously so that the browser has
		// time to create associated internal objects.
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.Show()
		}))
		capi.PostTask(capi.TidUi, task)
	}

	bwo.rootWin_.OnBrowserCreated(browser)
}

func (bwo *BrowserWindowOsr) Show() {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.Show()
		}))
		capi.PostTask(capi.TidUi, task)
		return
	}
	if bwo.browser_ == nil {
		return
	}
	if bwo.osr_hwnd_ != 0 && !win32api.IsWindowVisible(bwo.osr_hwnd_) {
		win32api.ShowWindow(bwo.osr_hwnd_, win32const.SwShow)
	}
	if bwo.hidden_ {
		bwo.browser_.GetHost().WasHidden(false)
		bwo.hidden_ = false
	}

	bwo.browser_.GetHost().SendFocusEvent(true)
}

func (bwo *BrowserWindowOsr) Hide() {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.Hide()
		}))
		capi.PostTask(capi.TidUi, task)
	}
	if bwo.browser_ == nil {
		return
	}
	bwo.browser_.GetHost().SendFocusEvent(false)
	if !bwo.hidden_ {
		bwo.browser_.GetHost().WasHidden(true)
		bwo.hidden_ = true
	}
}

func (bwo *BrowserWindowOsr) DoClose(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) bool {
	bwo.is_closing_ = true
	bwo.rootWin_.OnBrowserWindowClosing()

	return false
}

func (bwo *BrowserWindowOsr) GetResourceRequestHandler(
	self *capi.CRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	is_navigation int,
	is_download int,
	request_initiator string,
) (*capi.CResourceRequestHandlerT, bool) {
	return bwo.resourceManager.GetCResourceRequestHandlerT(), false
}

func (bw *BrowserWindowOsr) OnAddressChange(
	self *capi.CDisplayHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	url string,
) {
	if frame.IsMain() {
		if bw.rootWin_.edit_hwnd_ != 0 {
			win32api.SetWindowText(bw.rootWin_.edit_hwnd_, syscall.StringToUTF16Ptr(url))
		}
	}
}

func (bwo *BrowserWindowOsr) IsClosing() bool {
	return bwo.is_closing_
}

func (bwo *BrowserWindowOsr) SetDeviceScaleFactor(device_scale_factor float32) {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.SetDeviceScaleFactor(device_scale_factor)
		}))
		capi.PostTask(capi.TidUi, task)
		return
	}

	if bwo.device_scale_factor_ == device_scale_factor {
		return
	}
	bwo.device_scale_factor_ = device_scale_factor
	if bwo.browser_ != nil {
		bwo.browser_.GetHost().NotifyScreenInfoChanged()
		bwo.browser_.GetHost().WasResized()
	}
}

func (bwo *BrowserWindowOsr) SetFocus(focus bool) {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.SetFocus(focus)
		}))
		capi.PostTask(capi.TidUi, task)
		return
	}
	if bwo.osr_hwnd_ != 0 && focus {
		win32api.SetFocus(bwo.osr_hwnd_)
	}
}

func OsrWndProc(hWnd win32api.HWND, message win32api.UINT, wParam win32api.WPARAM, lParam win32api.LPARAM) win32api.LRESULT {
	bwo, ok := windowManager.GetBrowserWindowOsr(hWnd)
	if !ok {
		return win32api.DefWindowProc(hWnd, message, wParam, lParam)
	}
	msgId := win32const.MessageId(message)
	switch msgId {
	case win32const.WmLbuttondown, win32const.WmMbuttondown, win32const.WmRbuttondown,
		win32const.WmLbuttonup, win32const.WmMbuttonup, win32const.WmRbuttonup,
		win32const.WmMousemove, win32const.WmMouseleave, win32const.WmMousewheel:
		bwo.OnMouseEvent(msgId, wParam, lParam)

	case win32const.WmSize:
		bwo.OnSize()

	case win32const.WmSetfocus, win32const.WmKillfocus:
		bwo.SetFocus(win32const.MessageId(message) == win32const.WmSetfocus)

	case win32const.WmCapturechanged, win32const.WmCancelmode:
		bwo.OnCaptureLost()

	case win32const.WmSyschar, win32const.WmSyskeydown, win32const.WmSyskeyup,
		win32const.WmKeydown, win32const.WmKeyup, win32const.WmChar:
		bwo.OnKeyEvent(msgId, wParam, lParam)

	case win32const.WmPaint:
		bwo.OnWmPaint()
		return 0

	case win32const.WmErasebkgnd:
		// Erase the background when the browser does not exist.
		if bwo.browser_ != nil {
			return 0
		}

	case win32const.WmNcdestroy:
		windowManager.RemoveBrowserWindowOsr(bwo.osr_hwnd_)
		bwo.osr_hwnd_ = 0
	}

	return win32api.DefWindowProc(hWnd, message, wParam, lParam)
}

func (bwo *BrowserWindowOsr) OnWmPaint() {
	var ps win32api.Paintstruct
	win32api.BeginPaint(bwo.osr_hwnd_, &ps)
	win32api.EndPaint(bwo.osr_hwnd_, &ps)

	if bwo.browser_ != nil {
		bwo.browser_.GetHost().Invalidate(capi.PetView)
	}
}

func IsMouseEventFromTouch(message win32const.MessageId) bool {
	const MOUSEEVENTF_FROMTOUCH = 0xFF515700
	return (message >= win32const.WmMousefirst) && (message <= win32const.WmMouselast) &&
		(win32api.GetMessageExtraInfo()&MOUSEEVENTF_FROMTOUCH) == MOUSEEVENTF_FROMTOUCH
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (bwo *BrowserWindowOsr) OnMouseEvent(message win32const.MessageId, wParam win32api.WPARAM, lParam win32api.LPARAM) {
	if IsMouseEventFromTouch(message) {
		return
	}
	var browser_host *capi.CBrowserHostT
	if bwo.browser_ != nil {
		browser_host = bwo.browser_.GetHost()
	}
	var currentTime win32api.LONG = 0
	cancelPreviousClick := false

	switch message {
	case win32const.WmLbuttondown, win32const.WmRbuttondown, win32const.WmMbuttondown, win32const.WmMousemove, win32const.WmMouseleave:
		currentTime = win32api.GetMessageTime()
		x := win32api.GET_X_LPARAM(lParam)
		y := win32api.GET_Y_LPARAM(lParam)
		cancelPreviousClick =
			(abs(bwo.last_click_x_-x) > (win32api.GetSystemMetrics(win32const.SmCxdoubleclk) / 2)) ||
				(abs(bwo.last_click_y_-y) > (win32api.GetSystemMetrics(win32const.SmCydoubleclk) / 2)) ||
				((currentTime - bwo.last_click_time_) > win32api.LONG(win32api.GetDoubleClickTime()))
		if cancelPreviousClick &&
			(message == win32const.WmMousemove || message == win32const.WmMouseleave) {
			bwo.last_click_count_ = 1
			bwo.last_click_x_ = 0
			bwo.last_click_y_ = 0
			bwo.last_click_time_ = 0

		}
	}
	switch message {
	case win32const.WmLbuttondown, win32const.WmRbuttondown, win32const.WmMbuttondown:
		win32api.SetCapture(bwo.osr_hwnd_)
		win32api.SetFocus(bwo.osr_hwnd_)
		x := win32api.GET_X_LPARAM(lParam)
		y := win32api.GET_Y_LPARAM(lParam)
		if wParam&win32const.MkShift != 0 {
			bwo.current_mouse_pos_.X = win32api.LONG(x)
			bwo.current_mouse_pos_.Y = win32api.LONG(y)
			bwo.last_mouse_pos_.X = bwo.current_mouse_pos_.X
			bwo.last_mouse_pos_.Y = bwo.current_mouse_pos_.Y
			bwo.mouse_rotation_ = true
		} else {
			var btnType capi.CMouseButtonTypeT
			switch message {
			case win32const.WmLbuttondown:
				btnType = capi.MbtLeft
			case win32const.WmMbuttondown:
				btnType = capi.MbtMiddle
			case win32const.WmRbuttondown:
				btnType = capi.MbtRight
			}
			if !cancelPreviousClick && (btnType == bwo.last_click_button_) {
				bwo.last_click_count_++
			} else {
				bwo.last_click_count_ = 1
				bwo.last_click_x_ = x
				bwo.last_click_y_ = y
			}
			bwo.last_click_time_ = currentTime
			bwo.last_click_button_ = btnType
			if browser_host != nil {
				bwo.last_mouse_down_on_view_ = !bwo.IsOverPopupWidget(x, y)
				x, y = bwo.ApplyPopupOffset(x, y)
				var mouse_event capi.CMouseEventT
				mouse_event.SetX(x)
				mouse_event.SetY(y)
				mouse_event = DeviceToLogicalMouseEvent(mouse_event, bwo.device_scale_factor_)
				mouse_event.SetModifiers(uint32(GetCefMouseModifiers(wParam)))
				browser_host.SendMouseClickEvent(&mouse_event, btnType, false, bwo.last_click_count_)
			}
		}
	case win32const.WmLbuttonup, win32const.WmMbuttonup, win32const.WmRbuttonup:
		if win32api.GetCapture() == bwo.osr_hwnd_ {
			win32api.ReleaseCapture()
		}
		if bwo.mouse_rotation_ {
			// End rotation effect.
			bwo.mouse_rotation_ = false
			bwo.renderer_.SetSpin(0, 0)
			bwo.Invalidate()
		} else {
			x := win32api.GET_X_LPARAM(lParam)
			y := win32api.GET_Y_LPARAM(lParam)
			var btnType capi.CMouseButtonTypeT
			switch message {
			case win32const.WmLbuttonup:
				btnType = capi.MbtLeft
			case win32const.WmMbuttonup:
				btnType = capi.MbtMiddle
			case win32const.WmRbuttonup:
				btnType = capi.MbtRight
			}
			if browser_host != nil {
				if bwo.last_mouse_down_on_view_ && bwo.IsOverPopupWidget(x, y) &&
					bwo.GetPopupXoffset() != 0 && bwo.GetPopupYoffset() != 0 {
					break
				}
				x, y = bwo.ApplyPopupOffset(x, y)
				var mouse_event capi.CMouseEventT
				mouse_event.SetX(x)
				mouse_event.SetY(y)
				mouse_event = DeviceToLogicalMouseEvent(mouse_event, bwo.device_scale_factor_)
				mouse_event.SetModifiers(uint32(GetCefMouseModifiers(wParam)))
				browser_host.SendMouseClickEvent(&mouse_event, btnType, true, bwo.last_click_count_)
			}
		}
	case win32const.WmMousemove:
		x := win32api.GET_X_LPARAM(lParam)
		y := win32api.GET_Y_LPARAM(lParam)
		if bwo.mouse_rotation_ {
			// Apply rotation effect.
			bwo.current_mouse_pos_.X = win32api.LONG(x)
			bwo.current_mouse_pos_.Y = win32api.LONG(y)
			bwo.renderer_.IncrementSpin(
				float32(bwo.current_mouse_pos_.X-bwo.last_mouse_pos_.X),
				float32(bwo.current_mouse_pos_.Y-bwo.last_mouse_pos_.Y),
			)
			bwo.Invalidate()
			bwo.last_mouse_pos_.X = bwo.current_mouse_pos_.X
			bwo.last_mouse_pos_.Y = bwo.current_mouse_pos_.Y
		} else {
			if !bwo.mouse_tracking_ {
				// Start tracking mouse leave. Required for the WM_MOUSELEAVE event to
				// be generated.
				var tme win32api.Trackmouseevent
				tme.Size = win32api.DWORD(unsafe.Sizeof(tme))
				tme.Flags = win32const.TmeLeave
				tme.Track = bwo.osr_hwnd_
				if err := win32api.TrackMouseEvent(&tme); err != nil {
					log.Panicln("T583:", err)
				}
				bwo.mouse_tracking_ = true
			}

			if browser_host != nil {
				x, y = bwo.ApplyPopupOffset(x, y)
				var mouse_event capi.CMouseEventT
				mouse_event.SetX(x)
				mouse_event.SetY(y)
				mouse_event = DeviceToLogicalMouseEvent(mouse_event, bwo.device_scale_factor_)
				mouse_event.SetModifiers(uint32(GetCefMouseModifiers(wParam)))
				browser_host.SendMouseMoveEvent(&mouse_event, false)
			}
		}
	case win32const.WmMouseleave:
		if bwo.mouse_tracking_ {
			// Stop tracking mouse leave.
			var tme win32api.Trackmouseevent
			tme.Size = win32api.DWORD(unsafe.Sizeof(tme))
			tme.Flags = win32const.TmeLeave & win32const.TmeCancel
			tme.Track = bwo.osr_hwnd_
			if err := win32api.TrackMouseEvent(&tme); err != nil {
				log.Panicln("T607:", err)
			}
			bwo.mouse_tracking_ = false
		}
		if browser_host != nil {
			var p win32api.Point
			if err := win32api.GetCursorPos(&p); err != nil {
				log.Panicln("T614:", err)
			}
			win32api.ScreenToClient(bwo.osr_hwnd_, &p)

			var mouse_event capi.CMouseEventT
			mouse_event.SetX(int(p.X))
			mouse_event.SetY(int(p.Y))
			mouse_event = DeviceToLogicalMouseEvent(mouse_event, bwo.device_scale_factor_)
			mouse_event.SetModifiers(uint32(GetCefMouseModifiers(wParam)))
			browser_host.SendMouseMoveEvent(&mouse_event, true)
		}
	case win32const.WmMousewheel:
		if browser_host != nil {
			screen_point := win32api.Point{
				X: win32api.LONG(win32api.GET_X_LPARAM(lParam)),
				Y: win32api.LONG(win32api.GET_Y_LPARAM(lParam)),
			}
			scrolled_wnd := win32api.WindowFromPoint(screen_point)
			if scrolled_wnd != bwo.osr_hwnd_ {
				break
			}
			win32api.ScreenToClient(bwo.osr_hwnd_, &screen_point)
			delta := win32api.GET_WHEEL_DELTA_WPARAM(wParam)
			var mouse_event capi.CMouseEventT
			x, y := bwo.ApplyPopupOffset(int(screen_point.X), int(screen_point.Y))
			mouse_event.SetX(x)
			mouse_event.SetY(y)
			mouse_event = DeviceToLogicalMouseEvent(mouse_event, bwo.device_scale_factor_)
			mouse_event.SetModifiers(uint32(GetCefMouseModifiers(wParam)))
			var delta_x, delta_y int
			if IsKeyDown(win32const.VkShift) {
				delta_x = delta
			} else {
				delta_y = delta
			}
			browser_host.SendMouseWheelEvent(&mouse_event, delta_x, delta_y)
		}
	}
}

func (bwo *BrowserWindowOsr) GetRootScreenRect(
	self *capi.CRenderHandlerT,
	browser *capi.CBrowserT,
) (ret bool, rect capi.CRectT) {
	return false, rect
}

func (bwo *BrowserWindowOsr) GetViewRect(
	self *capi.CRenderHandlerT,
	browser *capi.CBrowserT,
) (rect capi.CRectT) {
	rect.SetX(0)
	rect.SetY(0)

	rect.SetWidth(DeviceToLogical(bwo.client_rect_.Width(), bwo.device_scale_factor_))
	if rect.Width() == 0 {
		rect.SetWidth(1)
	}

	rect.SetHeight(DeviceToLogical(bwo.client_rect_.Height(), bwo.device_scale_factor_))
	if rect.Height() == 0 {
		rect.SetHeight(1)
	}

	return rect
}

func (bwo *BrowserWindowOsr) GetScreenPoint(
	self *capi.CRenderHandlerT,
	browser *capi.CBrowserT,
	viewX int,
	viewY int,
) (ret bool, screenX int, screenY int) {
	if !win32api.IsWindow(bwo.osr_hwnd_) {
		return false, screenX, screenY
	}
	screen_pt := win32api.Point{
		X: win32api.LONG(LogicalToDevice(viewX, bwo.device_scale_factor_)),
		Y: win32api.LONG(LogicalToDevice(viewY, bwo.device_scale_factor_)),
	}
	win32api.ClientToScreen(bwo.osr_hwnd_, &screen_pt)
	screenX = int(screen_pt.X)
	screenY = int(screen_pt.Y)

	return true, screenX, screenY
}

func (bwo *BrowserWindowOsr) GetScreenInfo(
	self *capi.CRenderHandlerT,
	browser *capi.CBrowserT,
	screen_info capi.CScreenInfoT,
) (ret bool, outScreenInfo capi.CScreenInfoT) {
	if !win32api.IsWindow(bwo.osr_hwnd_) {
		return false, screen_info
	}
	view_rect := bwo.GetViewRect(self, browser)
	screen_info.SetDeviceScaleFactor(bwo.device_scale_factor_)
	screen_info.SetRect(view_rect)
	screen_info.SetAvailableRect(view_rect)

	return true, screen_info
}

func (bwo *BrowserWindowOsr) OnPaint(
	self *capi.CRenderHandlerT,
	browser *capi.CBrowserT,
	ctype capi.CPaintElementTypeT,
	dirtyRects []capi.CRectT,
	buffer unsafe.Pointer,
	width int,
	height int,
) {
	// OsrRenderHandlerWin::SetBrowser
	bwo.browser_ = browser
	if bwo.browser_ != nil && bwo.external_begin_frame_enabled {
		// Start the BeginFrame timer.
		bwo.Invalidate()
	}

	if bwo.painting_popup_ {
		bwo.renderer_.OnPaint(browser, ctype, dirtyRects, buffer, width, height)
		return
	}

	if bwo.hdc_ == 0 {
		bwo.hdc_, bwo.hrc_ = bwo.renderer_.EnableGL(bwo.osr_hwnd_)
	}

	//   ScopedGLContext scoped_gl_context(hdc_, hrc_, true);
	if err := win32api.WglMakeCurrent(bwo.hdc_, bwo.hrc_); err != nil {
		log.Panicln("T509:", err)
	}
	defer func() {
		if err := win32api.WglMakeCurrent(0, 0); err != nil {
			log.Panicln("T513:", err)
		}
		if err := win32api.SwapBuffers(bwo.hdc_); err != nil {
			log.Panicln("T516:", err)
		}
	}()

	bwo.renderer_.OnPaint(browser, ctype, dirtyRects, buffer, width, height)
	if ctype == capi.PetView && !bwo.renderer_.popup_rect_.IsEmpty() {
		bwo.painting_popup_ = true
		bwo.browser_.GetHost().Invalidate(capi.PetPopup)
		bwo.painting_popup_ = false
	}
	bwo.renderer_.Render()
}

func (bwo *BrowserWindowOsr) IsOverPopupWidget(x, y int) bool {
	rc := bwo.renderer_.popup_rect_
	popup_right := rc.X() + rc.Width()
	popup_bottom := rc.Y() + rc.Height()
	return (x >= rc.X()) && (x < popup_right) && (y >= rc.Y()) && (y < popup_bottom)
}

func (bwo *BrowserWindowOsr) GetPopupXoffset() int {
	return bwo.renderer_.original_popup_rect_.X() - bwo.renderer_.popup_rect_.X()
}
func (bwo *BrowserWindowOsr) GetPopupYoffset() int {
	return bwo.renderer_.original_popup_rect_.Y() - bwo.renderer_.popup_rect_.Y()
}

func (bwo *BrowserWindowOsr) ApplyPopupOffset(x, y int) (int, int) {
	if bwo.IsOverPopupWidget(x, y) {
		x += bwo.GetPopupXoffset()
		y += bwo.GetPopupYoffset()
	}
	return x, y
}

func (bwo *BrowserWindowOsr) Invalidate() {
	if bwo.begin_frame_pending_ {
		return
	}
	var delay_us float32 = float32((1.0 / float64(bwo.windowless_frame_rate)) * 1000000)
	bwo.TriggerBeginFrame(0, delay_us)
}

func (bwo *BrowserWindowOsr) TriggerBeginFrame(last_time_us uint64, delay_us float32) {
	if bwo.begin_frame_pending_ && !bwo.external_begin_frame_enabled {
		// Render immediately and then wait for the next call to Invalidate() or
		// On[Accelerated]Paint().
		bwo.begin_frame_pending_ = false
		bwo.Render()
		return
	}

	now := GetTimeNow()
	offset := float32(now - last_time_us)
	if offset > delay_us {
		offset = delay_us
	}
	if !bwo.begin_frame_pending_ {
		bwo.begin_frame_pending_ = true
	}

	task := cef.NewTask(cef.TaskFunc(func() {
		bwo.TriggerBeginFrame(now, delay_us)
	}))
	capi.PostDelayedTask(capi.TidUi, task, int64(offset/1000))

	if bwo.external_begin_frame_enabled && bwo.browser_ != nil {
		bwo.browser_.GetHost().SendExternalBeginFrame()
	}
}

func (bwo *BrowserWindowOsr) Render() {
	if bwo.hdc_ == 0 {
		bwo.hdc_, bwo.hrc_ = bwo.renderer_.EnableGL(bwo.osr_hwnd_)
	}

	//   ScopedGLContext scoped_gl_context(hdc_, hrc_, true);
	if err := win32api.WglMakeCurrent(bwo.hdc_, bwo.hrc_); err != nil {
		log.Panicln("T509:", err)
	}
	defer func() {
		if err := win32api.WglMakeCurrent(0, 0); err != nil {
			log.Panicln("T513:", err)
		}
		if err := win32api.SwapBuffers(bwo.hdc_); err != nil {
			log.Panicln("T516:", err)
		}
	}()

	bwo.renderer_.Render()
}

func (bwo *BrowserWindowOsr) OnSize() {
	// Keep |client_rect_| up to date.
	var rect win32api.Rect
	if err := win32api.GetClientRect(bwo.osr_hwnd_, &rect); err != nil {
		log.Panicln("T850:", err)
	}
	bwo.client_rect_.SetX(int(rect.Left))
	bwo.client_rect_.SetY(int(rect.Top))
	bwo.client_rect_.SetWidth(int(rect.Right - rect.Left))
	bwo.client_rect_.SetHeight(int(rect.Bottom - rect.Top))

	if bwo.browser_ != nil {
		bwo.browser_.GetHost().WasResized()
	}
}

func (bwo *BrowserWindowOsr) Setfocus(setFocus bool) {
	if bwo.browser_ != nil {
		bwo.browser_.GetHost().SendFocusEvent(setFocus)
	}
}

func (bwo *BrowserWindowOsr) OnCaptureLost() {
	if bwo.mouse_rotation_ {
		return
	}
	if bwo.browser_ != nil {
		bwo.browser_.GetHost().SendCaptureLostEvent()
	}
}

func (bwo *BrowserWindowOsr) OnKeyEvent(message win32const.MessageId, wParam win32api.WPARAM, lParam win32api.LPARAM) {
	if bwo.browser_ == nil {
		return
	}
	log.Printf("T896: %d, %x, %x\n", message, wParam, lParam)

	var event capi.CKeyEventT
	event.SetWindowsKeyCode(int(wParam))
	event.SetNativeKeyCode(int(lParam))
	if message == win32const.WmSyschar || message == win32const.WmSyskeydown || message == win32const.WmSyskeyup {
		event.SetIsSystemKey(1)
	}

	if message == win32const.WmKeydown || message == win32const.WmSyskeydown {
		event.SetType(capi.KeyeventRawkeydown)
	} else if message == win32const.WmKeyup || message == win32const.WmSyskeyup {
		event.SetType(capi.KeyeventKeyup)
	} else {
		event.SetType(capi.KeyeventChar)
	}

	event.SetModifiers(uint32(GetCefKeyboardModifiers(wParam, lParam)))
	// mimic alt-gr check behaviour from
	// src/ui/events/win/events_win_utils.cc: GetModifiersFromKeyState
	if (event.Type() == capi.KeyeventChar) && IsKeyDown(win32const.VkRmenu) {
		// reverse AltGr detection taken from PlatformKeyMap::UsesAltGraph
		// instead of checking all combination for ctrl-alt, just check current char
		current_layout := win32api.GetKeyboardLayout(0)

		// https://docs.microsoft.com/en-gb/windows/win32/api/winuser/nf-winuser-vkkeyscanexw
		// ... high-order byte contains the shift state,
		// which can be a combination of the following flag bits.
		// 2 Either CTRL key is pressed.
		// 4 Either ALT key is pressed.
		scan_res := win32api.VkKeyScanEx(win32api.WCHAR(wParam), current_layout)
		if ((scan_res >> 8) & 0xFF) == (2 | 4) { // ctrl-alt pressed
			modifiers := capi.CEventFlagsT(event.Modifiers())
			modifiers &= ^(capi.EventflagControlDown | capi.EventflagAltDown)
			modifiers |= capi.EventflagAltgrDown
			event.SetModifiers(uint32(modifiers))
		}
	}

	bwo.browser_.GetHost().SendKeyEvent(&event)

}

func (origin *BrowserWindowOsr) OnBeforePopup(
	self *capi.CLifeSpanHandlerT,
	originBrowser *capi.CBrowserT,
	originFrame *capi.CFrameT,
	target_url string,
	target_frame_name string,
	target_disposition capi.CWindowOpenDispositionT,
	user_gesture int,
	popupFeatures *capi.CPopupFeaturesT,
	windowInfo capi.CWindowInfoT,
	client *capi.CClientT,
	settings capi.CBrowserSettingsT,
	no_javascript_access bool,
) (
	ret bool,
	windowInfoOut capi.CWindowInfoT,
	clientOut *capi.CClientT,
	settingsOut capi.CBrowserSettingsT,
	extra_info *capi.CDictionaryValueT,
	no_javascript_accessOut bool,
) {
	return OnBeforePopup(origin, target_url, popupFeatures, windowInfo, settings, no_javascript_access)
}

func (bwo *BrowserWindowOsr) IsOsr() bool {
	return true
}

func (bwo *BrowserWindowOsr) ShowPopup(parentHwnd win32api.HWND, rect capi.CRectT) {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.ShowPopup(parentHwnd, rect)
		}))
		capi.PostTask(capi.TidUi, task)
		return
	}
	bwo.Create(parentHwnd, rect)

	// render_handler_->SetBrowser(browser_);
	if bwo.browser_ == nil {
		log.Panicln("T979:")
	}
	if bwo.browser_ != nil && bwo.external_begin_frame_enabled {
		// Start the BeginFrame timer.
		bwo.Invalidate()
	}

	bwo.browser_.GetHost().WasResized()
	bwo.Show()
}
