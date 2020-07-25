package main

import (
	"log"
	"net/url"
	"strings"
	"syscall"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type BrowserWindow struct {
	rootWin_    *RootWindowWin
	browser_    *capi.CBrowserT
	is_closing_ bool
	capi.RefToCClientT
	capi.RefToCLifeSpanHandlerT
	capi.RefToCLoadHandlerT
	capi.RefToCRequestHandlerT
	capi.RefToCResourceRequestHandlerT

	resourceManager ResourceManager
}

func NewBrowserWindow(rootWindow *RootWindowWin) *BrowserWindow {
	bw := &BrowserWindow{}
	bw.rootWin_ = rootWindow
	bw.resourceManager.rh = map[string]*capi.CResourceHandlerT{}

	capi.AllocCLifeSpanHandlerT().Bind(bw)
	capi.AllocCClientT().Bind(bw)
	capi.AllocCLoadHandlerT().Bind(bw)

	capi.AllocCRequestHandlerT().Bind(bw)
	capi.AllocCResourceRequestHandlerT().Bind(bw)

	return bw
}

func init() {
	var bw *BrowserWindow
	// capi.CClientT
	var _ capi.OnLoadingStateChangeHandler = bw
	var _ capi.GetLifeSpanHandlerHandler = bw
	var _ capi.CClientTGetLoadHandlerHandler = bw
	var _ capi.GetRequestHandlerHandler = bw

	// capi.CLoadHandlerT
	var _ capi.OnLoadingStateChangeHandler = bw

	// capi.CLifeSpanHandlerT
	var _ capi.OnBeforeCloseHandler = bw
	var _ capi.OnAfterCreatedHandler = bw
	var _ capi.DoCloseHandler = bw
	var _ capi.OnBeforePopupHandler = bw

	// capi.CRequestHandlerT
	var _ capi.CRequestHandlerTGetResourceRequestHandlerHandler = bw

	// capi.CResourceRequestHandlerT
	var _ capi.OnBeforeResourceLoadHandler = bw
	var _ capi.GetResourceHandlerHandler = bw
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

func (bw *BrowserWindow) GetLifeSpanHandler(*capi.CClientT) *capi.CLifeSpanHandlerT {
	return bw.GetCLifeSpanHandlerT()
}

func (bw *BrowserWindow) GetLoadHandler(*capi.CClientT) *capi.CLoadHandlerT {
	return bw.GetCLoadHandlerT()
}

func (bw *BrowserWindow) GetRequestHandler(*capi.CClientT) *capi.CRequestHandlerT {
	return bw.GetCRequestHandlerT()
}


func (bw *BrowserWindow) OnBeforeClose(self *capi.CLifeSpanHandlerT, browser *capi.CBrowserT) {
	// capi.QuitMessageLoop()

	bw.OnBrowserClosed(browser)

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
	bw.rootWin_.OnBrowserCreated(browser)
}

func (bw *BrowserWindow) DoClose(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) bool {
	log.Println("T83: DoClose")
	bw.OnBrowserClosing(browser)

	return false
}

func (old *BrowserWindow) OnBeforePopup(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
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

	settingsOut = settings
	rect := win32api.Rect{}
	if popupFeatures.XSet() {
		rect.Left = win32api.LONG(popupFeatures.X())
	}
	if popupFeatures.YSet() {
		rect.Top = win32api.LONG(popupFeatures.Y())
	}
	if popupFeatures.WidthSet() {
		rect.Right = rect.Left + win32api.LONG(popupFeatures.Width())
	}
	if popupFeatures.HeightSet() {
		rect.Bottom = rect.Top + win32api.LONG(popupFeatures.Height())
	}

	_, bw := windowManager.CreateRootWindow(true, true, rect, false, false, &settingsOut)

	ret = false
	clientOut = bw.GetCClientT()
	windowInfoOut = windowInfo

	temp_hwnd_ := windowManager.GetTempWindow()

	windowInfoOut.SetParentWindow(capi.ToCWindowHandleT(syscall.Handle(temp_hwnd_)))
	windowInfoOut.SetStyle(
		win32const.WsChild | win32const.WsClipchildren |
		 win32const.WsClipsiblings | win32const.WsTabstop | win32const.WsVisible)
	exStyle := windowInfoOut.ExStyle()
	windowInfoOut.SetExStyle(exStyle | win32const.WsExNoactivate)

	return ret, windowInfoOut, clientOut, settingsOut, extra_info, no_javascript_access
}

func (bw *BrowserWindow) OnBrowserClosing(browser *capi.CBrowserT) {
	bw.is_closing_ = true

	bw.rootWin_.OnBrowserWindowClosing()
}

func (bw *BrowserWindow) OnBrowserClosed(browser *capi.CBrowserT) {
	bw.rootWin_.OnBrowserWindowDestroyed()
	bw.GetCClientT().UnbindAll()
	bw.GetCLifeSpanHandlerT().UnbindAll()
	bw.GetCLoadHandlerT().UnbindAll()
	bw.GetCRequestHandlerT().UnbindAll()
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

func (bw *BrowserWindow) IsClosing() bool {
	return bw.is_closing_
}

func (bw *BrowserWindow) GetResourceRequestHandler(
	self *capi.CRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	is_navigation int,
	is_download int,
	request_initiator string,
) (*capi.CResourceRequestHandlerT, bool) {
	return bw.GetCResourceRequestHandlerT(), false
}

func (bw *BrowserWindow) OnBeforeResourceLoad(
	self *capi.CResourceRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	callback *capi.CRequestCallbackT,
) (ret capi.CReturnValueT) {
	return capi.RvContinue
}

const kTestOrigin = "http://tests/"
const kTestGetSourcePage = "get_source.html"

func (bw *BrowserWindow) GetResourceHandler(
	self *capi.CResourceRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
) (handler *capi.CResourceHandlerT) {
	u, err := url.Parse(request.GetUrl())
	if err != nil {
		log.Println("T285:", request.GetUrl(), err)
		return handler
	}
	// log.Println("T300:",u.Scheme, u.Host, kTestOrigin)
	if rh, ok := bw.resourceManager.rh[request.GetUrl()]; ok {
		handler = rh
	}

	return handler
}

type ResourceManager struct {
	rh map[string]*capi.CResourceHandlerT
}

func (rm *ResourceManager) AddStringResource(url, mime, content string) {
	rh := &ResourceHandler{capi.RefToCResourceHandlerT{}, nil, []byte(content), mime, 0}
	cefHandler := capi.AllocCResourceHandlerT().Bind(rh)
	rm.rh[url] = cefHandler
}

type ResourceHandler struct {
	capi.RefToCResourceHandlerT

	url *url.URL

	text []byte
	mime string
	next int
}

func init() {
	var rh *ResourceHandler
	// capi.CResourceHandlerT
	var _ capi.ProcessRequestHandler = rh
	var _ capi.GetResponseHeadersHandler = rh
	var _ capi.CResourceHandlerTReadHandler = rh
}

func (rm *ResourceHandler) ProcessRequest(
	self *capi.CResourceHandlerT,
	request *capi.CRequestT,
	callback *capi.CCallbackT,
) bool {
	u, err := url.Parse(request.GetUrl())
	if err != nil {
		capi.Panicf("L305: Error")
	}
	rm.url = u

	capi.Logf("L309: %s", rm.url)
	callback.Cont()

	return true
}

func (rm *ResourceHandler) GetResponseHeaders(
	self *capi.CResourceHandlerT,
	response *capi.CResponseT,
) (int64, string) {
	capi.Logf("L391: %s", rm.url.Path)
	response.SetMimeType(rm.mime)
	response.SetStatus(200)
	response.SetStatusText("OK")

	h := cef.NewStringMultimap()
	capi.StringMultimapAppend(h.CefObject(), "Content-Type", rm.mime+"; charset=utf-8")
	response.SetHeaderMap(h.CefObject())

	return int64(len(rm.text)), ""
}

// ReadResponse method is deprecated from cef 75
func (rm *ResourceHandler) Read(
	self *capi.CResourceHandlerT,
	data_out []byte,
	callback *capi.CResourceReadCallbackT,
) (bool, int) {
	l := min(len(data_out), len(rm.text) - rm.next)
	for i := 0; i < l; i++ {
		data_out[i] = rm.text[rm.next+i]
	}

	rm.next = rm.next + l
	capi.Logf("L409: %d, %d, %d", len(rm.text), l, rm.next)
	ret := true
	if l <= 0 {
		ret = false
	}
	return ret, l
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}


func (bw *BrowserWindow) GetSource() {
	mySv := myStringVisitor{
		browserWindow: bw,
	}
	sv := capi.AllocCStringVisitorT()
	sv.Bind(&mySv)
	bw.browser_.GetMainFrame().GetSource(sv)

}

type myStringVisitor struct {
	browserWindow *BrowserWindow
}

func init() {
	var _ capi.CStringVisitorTVisitHandler = (*myStringVisitor)(nil)
}

func (sv *myStringVisitor) Visit(self *capi.CStringVisitorT, cstring string) {
	s := strings.Replace(cstring, ">", "&gt;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	ss := "<html><meta charset=\"utf-8\"><body bgcolor=\"white\">Source:<pre>" + s + "</pre></body></html>"
	log.Println("T761:", ss)

	sv.browserWindow.resourceManager.AddStringResource(kTestOrigin + kTestGetSourcePage, "text/html", ss)
	sv.browserWindow.browser_.GetMainFrame().LoadUrl(kTestOrigin + kTestGetSourcePage)
}
