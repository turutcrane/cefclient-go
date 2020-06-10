package main

import (
	"log"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type BrowserWindow struct {
	rootWin_ *RootWindowWin
	browser_ *capi.CBrowserT
	capi.RefToCClientT
}

func NewBrowserWindow(rootWindow *RootWindowWin) *BrowserWindow {
	bw := &BrowserWindow{}
	bw.rootWin_ = rootWindow

	life_span_handler := capi.AllocCLifeSpanHandlerT().Bind(bw)
	capi.AllocCClientT().Bind(bw)
	// defer client.SetCClientT(nil)
	bw.GetCClientT().AssocLifeSpanHandlerT(life_span_handler)

	load_handler := capi.AllocCLoadHandlerT().Bind(bw)
	bw.GetCClientT().AssocLoadHandlerT(load_handler)

	return bw
}

// capi.CClientT
func init() {
	var _ capi.OnLoadingStateChangeHandler = &BrowserWindow{}
}

func (bw *BrowserWindow) OnLoadingStateChange(
	self *capi.CLoadHandlerT,
	browser *capi.CBrowserT,
	isLoading bool,
	canGoBack bool,
	canGoForward bool,
) {
	log.Println("T198:", isLoading, canGoBack, canGoForward)
	rootWin := bw.rootWin_
	win32api.EnableWindow(rootWin.back_hwnd_, canGoBack)
	win32api.EnableWindow(rootWin.forward_hwnd_, canGoForward)
	win32api.EnableWindow(rootWin.reload_hwnd_, !isLoading)
	win32api.EnableWindow(rootWin.stop_hwnd_, isLoading)
	win32api.EnableWindow(rootWin.edit_hwnd_, true)
}

// capi.CLifeSpanHandlerT
func init() {
	var _ capi.OnBeforeCloseHandler = &BrowserWindow{}
	var _ capi.OnAfterCreatedHandler = &BrowserWindow{}
}

func (bw *BrowserWindow) OnBeforeClose(self *capi.CLifeSpanHandlerT, brwoser *capi.CBrowserT) {
	log.Println("T188:", "OnBeforeClose: QuitMessageLoop")
	capi.QuitMessageLoop()
}

func (bw *BrowserWindow) OnAfterCreated(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) {
	log.Println("T197:", "OnAfterCreated")
	if bw.browser_ == nil {
		bw.browser_ = browser
	} else {
		log.Println("T71:", "OnAfterCreated")
	}
}

func (bw *BrowserWindow) CreateBrowser(
	parentHandle capi.CWindowHandleT,
	rect *capi.CRectT,
	settings *capi.CBrowserSettingsT,
	extra_info *capi.CDictionaryValueT,
	request_context *capi.CRequestContextT,
) {
	windowInfo := &capi.CWindowInfoT{}
	windowInfo.SetParentWindow(parentHandle)
	windowInfo.SetX(rect.X())
	windowInfo.SetY(rect.Y())
	windowInfo.SetWidth(rect.Width())
	windowInfo.SetHeight(rect.Height())
	windowInfo.SetStyle(win32const.WsChild | win32const.WsClipchildren | win32const.WsClipsiblings | win32const.WsTabstop | win32const.WsVisible)

	capi.BrowserHostCreateBrowser(
		windowInfo,
		bw.GetCClientT(),
		*config.initial_url,
		settings,
		extra_info,
		request_context,
	)
}

func (bw *BrowserWindow) SetFocus(focus bool) {
	if bw.browser_ != nil {
		bw.browser_.GetHost().SetFocus(focus)
	}
}

func (bw *BrowserWindow) Hide() {
	hwnd := bw.GetWindowHandle()
	if hwnd != 0 {
		win32api.SetWindowPos(hwnd, 0, 0, 0, 0, 0,
			win32const.SwpNozorder|win32const.SwpNomove|win32const.SwpNoactivate)
	}

}

func (bw *BrowserWindow) Show() {
	hwnd := bw.GetWindowHandle()
	if hwnd != 0 && !win32api.IsWindowVisible(hwnd) {
		win32api.ShowWindow(hwnd, win32const.SwShow)
	}
}

func (bw *BrowserWindow) GetWindowHandle() win32api.HWND {
	if bw.browser_ != nil {
		h := bw.browser_.GetHost().GetWindowHandle()
		return win32api.HWND(uintptr(unsafe.Pointer(h)))
	}
	return 0
}

func (bw *BrowserWindow) SetBound(x, y int, width, height uint32) {
	hwnd := bw.GetWindowHandle()
	if hwnd != 0 {
		win32api.SetWindowPos(hwnd, 0, x, y, int(width), int(height), win32const.SwpNozorder)
	}
}
