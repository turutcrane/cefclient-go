package main

import (
	"log"
	"syscall"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type BrowserWindowOsr struct {
	rootWin_    *RootWindowWin
	browser_    *capi.CBrowserT
	is_closing_ bool
	is_osr_     bool // これはいらないかも type assersion でいいかも。
	hidden_     bool

	resourceManager ResourceManager

	osr_hwnd_                        win32api.HWND
	show_update_rect             bool
	external_begin_frame_enabled bool
	windowless_frame_rate        int
	background_color             capi.CColorT

	client_rect_ capi.CRectT

	capi.RefToCClientT
	capi.RefToCLifeSpanHandlerT
	capi.RefToCLoadHandlerT

	// capi.RefToCRequestHandlerT
	// capi.RefToCDisplayHandlerT

}

func init() {
	// capi.CClientT
	var _ capi.GetLifeSpanHandlerHandler = (*BrowserWindowOsr)(nil)
	var _ capi.CClientTGetLoadHandlerHandler = (*BrowserWindowOsr)(nil)

	// capi.CLifeSpanHandlerT
	var _ capi.OnBeforeCloseHandler = (*BrowserWindowOsr)(nil)
	var _ capi.OnAfterCreatedHandler = (*BrowserWindowOsr)(nil)
	var _ capi.DoCloseHandler = (*BrowserWindowOsr)(nil)

	// capi.CLoadHandlerT
	var _ capi.OnLoadingStateChangeHandler = (*BrowserWindowOsr)(nil)
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
	bwo.is_osr_ = rootWindow.with_osr_ // いらんのじゃね？

	bwo.show_update_rect = show_update_rect
	bwo.external_begin_frame_enabled = external_begin_frame_enabled
	bwo.windowless_frame_rate = windowless_frame_rate
	bwo.background_color = background_color

	capi.AllocCLifeSpanHandlerT().Bind(bwo)
	capi.AllocCClientT().Bind(bwo)

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

	// capi.BrowserHostCreateBrowser(
	// 	windowInfo, bwo.GetCCl
	// )
}

func (bwo *BrowserWindowOsr) Create(
	parent_hwnd win32api.HWND,
	rect capi.CRectT,
) {
	window_class := "Client_OsrWindow"

	hInst, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T83:", err)
	}
	background_brush := win32api.CreateSolidBrush(
		RGB(CefColorGetR(bwo.background_color),
			CefColorGetG(bwo.background_color),
			CefColorGetB(bwo.background_color)),
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


func (bw *BrowserWindowOsr) OnLoadingStateChange(
	self *capi.CLoadHandlerT,
	browser *capi.CBrowserT,
	isLoading bool,
	canGoBack bool,
	canGoForward bool,
) {
	// log.Println("T198:", isLoading, canGoBack, canGoForward)
	rootWin := bw.rootWin_

	rootWin.OnLoadingStateChange(isLoading, canGoBack, canGoForward)
}

func (bwo *BrowserWindowOsr) GetLifeSpanHandler(*capi.CClientT) *capi.CLifeSpanHandlerT {
	return bwo.GetCLifeSpanHandlerT()
}

func (bwo *BrowserWindowOsr) GetLoadHandler(*capi.CClientT) *capi.CLoadHandlerT {
	return bwo.GetCLoadHandlerT()
}

func (bwo *BrowserWindowOsr) OnBeforeClose(self *capi.CLifeSpanHandlerT, browser *capi.CBrowserT) {

	// browser_ = nullptr;
	// render_handler_->SetBrowser(nullptr);

	// OsrWindowWin::Destroy()
	// render_handler_.reset();
	// // Destroy the native window.
	win32api.DestroyWindow(bwo.osr_hwnd_);
	// ime_handler_.reset();
	// hwnd_ = NULL;

	bwo.rootWin_.OnBrowserWindowDestroyed()
	bwo.GetCClientT().UnbindAll()
	bwo.GetCLifeSpanHandlerT().UnbindAll()
	bwo.GetCLoadHandlerT().UnbindAll()
	// bwo.GetCRequestHandlerT().UnbindAll()
	// bwo.GetCDisplayHandlerT().UnbindAll()

}

func (bwo *BrowserWindowOsr) OnAfterCreated(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) {

	log.Println("T195:", "OnAfterCreated")
	if bwo.browser_ == nil {
		bwo.browser_ = browser
	} else {
		log.Println("T99:", "OnAfterCreated")
	}

	// if (hwnd_) {
	// // The native window will already exist for non-popup browsers.
	// EnsureRenderHandler();
	// render_handler_->SetBrowser(browser);
	// }

	if (bwo.osr_hwnd_ != 0) {
		// Show the browser window. Called asynchronously so that the browser has
		// time to create associated internal objects.
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.Show()
		}))
		capi.PostTask(capi.TidUi, task);
	}

	bwo.rootWin_.OnBrowserCreated(browser)
}

func (bwo *BrowserWindowOsr) Show() {
	if !capi.CurrentlyOn(capi.TidUi) {
		task := cef.NewTask(cef.TaskFunc(func() {
			bwo.Show()
		}))
		capi.PostTask(capi.TidUi, task)
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

func (bwo *BrowserWindowOsr) DoClose(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) bool {
	log.Println("T83: DoClose")

	bwo.is_closing_ = true
	bwo.rootWin_.OnBrowserWindowClosing()
	
	return false
}
