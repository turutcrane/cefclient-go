package main

import (
	"log"
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
	capi.RefToCDisplayHandlerT

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

	capi.AllocCDisplayHandlerT().Bind(bw)

	capi.AllocCResponseFilterT().Bind(bw)

	return bw
}

func init() {
	var bw *BrowserWindow
	// capi.CClientT
	var _ capi.OnLoadingStateChangeHandler = bw
	var _ capi.GetLifeSpanHandlerHandler = bw
	var _ capi.CClientTGetLoadHandlerHandler = bw
	var _ capi.GetRequestHandlerHandler = bw
	var _ capi.GetDisplayHandlerHandler = bw

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

	// capi.CDisplayHandlerT
	var _ capi.OnAddressChangeHandler = bw

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

func (bw *BrowserWindow) GetDisplayHandler(*capi.CClientT) *capi.CDisplayHandlerT {
	return bw.GetCDisplayHandlerT()
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
	bw.GetCDisplayHandlerT().UnbindAll()
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
	// log.Println("T295:", request.GetUrl(), request.GetIdentifier())
	if request.GetUrl() == kTestOrigin+kTestRequestPage {
		bw.resourceManager.AddStreamResource(request)
	}
	return capi.RvContinue
}

const kTestOrigin = "http://tests/"
const kTestGetSourcePage = "get_source.html"
const kTestGetTextPage = "get_text.html"
const kTestRequestPage = "request.html"
const kTestPluginInfoPage = "plugin_info.html"

func (bw *BrowserWindow) GetResourceHandler(
	self *capi.CResourceRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
) (handler *capi.CResourceHandlerT) {
	// log.Println("T308:", request.GetUrl(), request.GetIdentifier())

	if rh, ok := bw.resourceManager.rh[request.GetUrl()]; ok {
		handler = rh
	}
	return handler
}

type ResourceManager struct {
	rh map[string]*capi.CResourceHandlerT
}

func (rm *ResourceManager) AddStringResource(url, mime, content string) {
	rh := &StringResourceHandler{url, []byte(content), mime, 0}
	cefHandler := capi.AllocCResourceHandlerT().Bind(rh)
	rm.rh[url] = cefHandler
}

type StringResourceHandler struct {
	url  string
	text []byte
	mime string
	next int
}

func init() {
	var rh *StringResourceHandler
	// capi.CResourceHandlerT
	var _ capi.OpenHandler = rh
	var _ capi.GetResponseHeadersHandler = rh
	var _ capi.CResourceHandlerTReadHandler = rh
}

func (rm *StringResourceHandler) Open(
	self *capi.CResourceHandlerT,
	request *capi.CRequestT,
	callback *capi.CCallbackT,
) (ret, handle_request bool) {
	return true, true
}

func (rm *StringResourceHandler) GetResponseHeaders(
	self *capi.CResourceHandlerT,
	response *capi.CResponseT,
) (int64, string) {
	response.SetMimeType(rm.mime)
	response.SetStatus(200)
	response.SetStatusText("OK")

	h := cef.NewStringMultimap()
	capi.StringMultimapAppend(h.CefObject(), "Content-Type", rm.mime+"; charset=utf-8")
	response.SetHeaderMap(h.CefObject())

	rm.next = 0
	return int64(len(rm.text)), ""
}

func (rm *StringResourceHandler) Read(
	self *capi.CResourceHandlerT,
	data_out []byte,
	callback *capi.CResourceReadCallbackT,
) (bool, int) {
	l := min(len(data_out), len(rm.text)-rm.next)
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

func GetDumpResponse(request *capi.CRequestT) (stream *capi.CStreamReaderT, responseHeaderMap *cef.StringMultimap) {
	responseHeaderMap = cef.NewStringMultimap()
	headerMap := cef.NewStringMultimap()
	request.GetHeaderMap(headerMap.CefObject())
	n := capi.StringMultimapFindCount(headerMap.CefObject(), "origin")
	for i := int64(0); i < n; i++ {
		if ok, value := capi.StringMultimapEnumerate(headerMap.CefObject(), "origin", i); ok {
			capi.StringMultimapAppend(headerMap.CefObject(), "Access-Control-Allow-Origin", value)
		}
	}
	if n > 0 {
		capi.StringMultimapAppend(headerMap.CefObject(), "Access-Control-Allow-Headers", "My-Custom-Header")
	}
	dump := DumpRequestContents(request)
	content := "<html><body bgcolor=\"white\"><pre>" + dump + "</pre></body></html>"
	stream = capi.StreamReaderCreateForData([]byte(content))
	return stream, responseHeaderMap
}

func DumpRequestContents(request *capi.CRequestT) (dump string) {
	dump += "URL:" + request.GetUrl() + "\n"
	dump += "Method: " + request.GetMethod() + "\n"

	headerMap := cef.NewStringMultimap()
	request.GetHeaderMap(headerMap.CefObject())
	n := capi.StringMultimapSize(headerMap.CefObject())
	for i := int64(n); i < n; i++ {
		if ok, key := capi.StringMultimapKey(headerMap.CefObject(), i); ok {
			dump += "\t" + key + ":"
		}
		if ok, value := capi.StringMultimapValue(headerMap.CefObject(), i); ok {
			dump += value + "\n"
		}
	}
	postData := request.GetPostData()
	if postData != nil {
		elementCount := postData.GetElementCount()
		if elementCount > 0 {
			elements := postData.GetElements()
			for _, e := range elements {
				switch e.GetType() {
				case capi.PdeTypeBytes:
					dump += "\tBytes: "
					n := e.GetBytesCount()
					if n == 0 {
						dump += "(empty)\n"
					} else {
						dump += string(cef.PostElementGetBytes(e)) + "\n"
					}
				case capi.PdeTypeFile:
					dump += "\tFile: " + e.GetFile()
				}
			}
		}
	}
	return dump
}

func (rm *ResourceManager) AddStreamResource(request *capi.CRequestT) {
	url := request.GetUrl()
	stream, header_map := GetDumpResponse(request)
	rh := &StreamResourceHandler{url, stream, header_map, "text/html", 200, "OK"}
	cefHandler := capi.AllocCResourceHandlerT().Bind(rh)
	rm.rh[url] = cefHandler
}

type StreamResourceHandler struct {
	url    string
	stream *capi.CStreamReaderT

	header_map_  *cef.StringMultimap
	mime_type_   string
	status_code_ int
	status_text_ string
}

func init() {
	var rh *StreamResourceHandler
	// capi.CResourceHandlerT
	var _ capi.OpenHandler = rh
	var _ capi.GetResponseHeadersHandler = rh
	var _ capi.CResourceHandlerTReadHandler = rh
}

func (rm *StreamResourceHandler) Open(
	self *capi.CResourceHandlerT,
	request *capi.CRequestT,
	callback *capi.CCallbackT,
) (ret, handle_request bool) {
	log.Println("T482:")
	return true, true
}

func (rm *StreamResourceHandler) GetResponseHeaders(
	self *capi.CResourceHandlerT,
	response *capi.CResponseT,
) (response_length int64, redirectUrl string) {
	response.SetMimeType(rm.mime_type_)
	response.SetStatus(rm.status_code_)
	response.SetStatusText(rm.status_text_)

	h := rm.header_map_
	if h == nil {
		h = cef.NewStringMultimap()
	}
	capi.StringMultimapAppend(h.CefObject(), "Content-Type", rm.mime_type_+"; charset=utf-8")
	response.SetHeaderMap(h.CefObject())

	return -1, ""
}

func (rm *StreamResourceHandler) Read(
	self *capi.CResourceHandlerT,
	data_out []byte,
	callback *capi.CResourceReadCallbackT,
) (read bool, bytes_read int) {
	bytes_to_read := len(data_out)
	var count int

	for ok := true; ok; ok = (count > 0 && bytes_read < bytes_to_read) {
		dp := unsafe.Pointer(&data_out[bytes_read:][0])
		count = int(rm.stream.Read(dp, 1, int64(bytes_to_read-bytes_read)))
		bytes_read += count
	}
	return bytes_read > 0, bytes_read
}

func (bw *BrowserWindow) GetSource() {
	url := kTestOrigin + kTestGetSourcePage
	mySv := myStringVisitor{
		f: func(c string) {
			bw.resourceManager.AddStringResource(url, "text/html", c)
			bw.browser_.GetMainFrame().LoadUrl(url)
		},
	}
	sv := capi.AllocCStringVisitorT().Bind(&mySv)
	bw.browser_.GetMainFrame().GetSource(sv)
}

type myStringVisitor struct {
	f func(content string)
}

func init() {
	var _ capi.CStringVisitorTVisitHandler = (*myStringVisitor)(nil)
}

func (sv *myStringVisitor) Visit(self *capi.CStringVisitorT, cstring string) {
	s := strings.Replace(cstring, ">", "&gt;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	ss := "<html><meta charset=\"utf-8\"><body bgcolor=\"white\">Source:<pre>" + s + "</pre></body></html>"
	// log.Println("T761:", ss)

	sv.f(ss)
}

func (bw *BrowserWindow) GetText() {
	url := kTestOrigin + kTestGetTextPage
	mySv := myStringVisitor{
		f: func(c string) {
			bw.resourceManager.AddStringResource(url, "text/html", c)
			bw.browser_.GetMainFrame().LoadUrl(url)
		},
	}
	sv := capi.AllocCStringVisitorT().Bind(&mySv)
	bw.browser_.GetMainFrame().GetText(sv)
}

func (bw *BrowserWindow) OnAddressChange(
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

type myPluginInfoVisitor struct {
	// capi.RefToCWebPluginInfoVisitorT
	html    string
	browser *BrowserWindow
}

func init() {
	var _ capi.CWebPluginInfoVisitorTVisitHandler = (*myPluginInfoVisitor)(nil)
}

func (v *myPluginInfoVisitor) Visit(
	self *capi.CWebPluginInfoVisitorT,
	info *capi.CWebPluginInfoT,
	count int,
	total int,
) bool {
	name := info.GetName()
	desc := info.GetDescription()
	ver := info.GetVersion()
	path := info.GetPath()
	log.Println("T592:", count, total, name, desc, ver, path)
	v.html += "\n<br/><br/>Name: " + name +
		"\n<br/>Description: " + desc +
		"\n<br/>Version: " + ver +
		"\n<br/>Path: " + path
	if count+1 >= total {
		v.html += "\n</body></html>"
		url := kTestOrigin + kTestPluginInfoPage
		v.browser.resourceManager.AddStringResource(url, "text/html", v.html)
		v.browser.browser_.GetMainFrame().LoadUrl(url)
	}
	return true
}
