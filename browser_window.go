package main

import (
	"log"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/cefingo/message_router"
	"github.com/turutcrane/win32api"
)

type BrowserWindowStd struct {
	rootWin_        *RootWindowWin
	// browser_        *capi.CBrowserT
	capi.RefToCBrowserT
	is_closing_     bool
	resourceManager *ResourceManager

	capi.RefToCClientT
	capi.RefToCLifeSpanHandlerT
	capi.RefToCLoadHandlerT
	capi.RefToCRequestHandlerT
	capi.RefToCDisplayHandlerT
}

func NewBrowserWindowStd(rootWindow *RootWindowWin) *BrowserWindowStd {
	bw := &BrowserWindowStd{}
	bw.rootWin_ = rootWindow
	bw.resourceManager = NewResourceManager()

	capi.AllocCLifeSpanHandlerT().Bind(bw)
	capi.AllocCClientT().Bind(bw)
	capi.AllocCLoadHandlerT().Bind(bw)

	capi.AllocCRequestHandlerT().Bind(bw)
	capi.AllocCResourceRequestHandlerT().Bind(&bw.resourceManager)

	capi.AllocCDisplayHandlerT().Bind(bw)
	capi.AllocCResponseFilterT().Bind(bw)

	return bw
}

func init() {
	var bw *BrowserWindowStd
	// capi.CClientT
	var _ capi.CClientTAccessor = bw
	var _ capi.GetLifeSpanHandlerHandler = bw
	var _ capi.CClientTGetLoadHandlerHandler = bw
	var _ capi.GetRequestHandlerHandler = bw
	var _ capi.GetDisplayHandlerHandler = bw
	var _ capi.CClientTOnProcessMessageReceivedHandler = bw

	// capi.CLifeSpanHandlerT
	var _ capi.OnBeforeCloseHandler = bw
	var _ capi.OnAfterCreatedHandler = bw
	var _ capi.DoCloseHandler = bw
	var _ capi.OnBeforePopupHandler = bw

	// capi.CLoadHandlerT
	var _ capi.OnLoadingStateChangeHandler = bw

	// capi.CRequestHandlerT
	var _ capi.CRequestHandlerTGetResourceRequestHandlerHandler = bw
	var _ capi.OnOpenUrlfromTabHandler = bw
	var _ capi.OnBeforeBrowseHandler = bw

	// capi.CDisplayHandlerT
	var _ capi.OnAddressChangeHandler = bw
}

func (bw *BrowserWindowStd) OnLoadingStateChange(
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

func (bw *BrowserWindowStd) GetLifeSpanHandler(*capi.CClientT) *capi.CLifeSpanHandlerT {
	return bw.GetCLifeSpanHandlerT()
}

func (bw *BrowserWindowStd) GetLoadHandler(*capi.CClientT) *capi.CLoadHandlerT {
	return bw.GetCLoadHandlerT()
}

func (bw *BrowserWindowStd) GetRequestHandler(*capi.CClientT) *capi.CRequestHandlerT {
	return bw.GetCRequestHandlerT()
}

func (bw *BrowserWindowStd) GetDisplayHandler(*capi.CClientT) *capi.CDisplayHandlerT {
	return bw.GetCDisplayHandlerT()
}

func (bw *BrowserWindowStd) OnBeforeClose(self *capi.CLifeSpanHandlerT, browser *capi.CBrowserT) {
	// capi.QuitMessageLoop()
	bw.OnBrowserClosed(browser)
}

func (bw *BrowserWindowStd) OnAfterCreated(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) {

	// ClientHandler::NotifyBrowserCreated
	if bw.GetCBrowserT() == nil {
		bw.TakeOverCBrowserT(browser) 
	} else {
		log.Println("T71:", "OnAfterCreated, not set bw.browser_")
	}
	bw.rootWin_.OnBrowserCreated(browser)
}

func (bw *BrowserWindowStd) DoClose(
	self *capi.CLifeSpanHandlerT,
	browser *capi.CBrowserT,
) bool {
	bw.is_closing_ = true
	bw.rootWin_.OnBrowserWindowClosing()

	return false
}

func (origin *BrowserWindowStd) OnBeforePopup(
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

func OnBeforePopup(
	origin BrowserWindow,
	target_url string,
	popupFeatures *capi.CPopupFeaturesT,
	windowInfo capi.CWindowInfoT,
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

	config := mainConfig
	config.main_url = target_url
	config.use_windowless_rendering = origin.IsOsr()
	rw := windowManager.CreateRootWindow(config, true, rect, &settingsOut)

	ret = false
	clientOut = rw.browser_window_.GetCClientT()
	windowInfoOut = windowInfo

	temp_hwnd_ := windowManager.GetTempWindow()
	if origin.IsOsr() {
		windowInfoOut.SetWindowlessRenderingEnabled(true)
		bwo := origin.(*BrowserWindowOsr)
		windowInfoOut.SetExternalBeginFrameEnabled(bwo.external_begin_frame_enabled)
		windowInfoOut.SetParentWindow(capi.ToCWindowHandleT(syscall.Handle(temp_hwnd_)))
	} else {
		windowInfoOut.SetParentWindow(capi.ToCWindowHandleT(syscall.Handle(temp_hwnd_)))
	}
	windowInfoOut.SetStyle(
		win32api.WsChild | win32api.WsClipchildren |
			win32api.WsClipsiblings | win32api.WsTabstop | win32api.WsVisible)

	// Don't activate the hidden browser on creation.
	exStyle := windowInfoOut.ExStyle()
	windowInfoOut.SetExStyle(exStyle | win32api.WsExNoactivate)

	return ret, windowInfoOut, clientOut, settingsOut, extra_info, no_javascript_access

}

func (bw *BrowserWindowStd) GetCBrowserT() *capi.CBrowserT {
	return bw.RefToCBrowserT.GetCBrowserT()
}

func (bw *BrowserWindowStd) IsOsr() bool {
	return false
}

func (bw *BrowserWindowStd) GetResourceManager() *ResourceManager {
	return bw.resourceManager
}

func (bw *BrowserWindowStd) OnBrowserClosed(browser *capi.CBrowserT) {
	bw.rootWin_.OnBrowserWindowDestroyed()
	bw.UnrefCBrowserT()
	bw.GetCClientT().UnbindAll()
	bw.GetCLifeSpanHandlerT().UnbindAll()
	bw.GetCLoadHandlerT().UnbindAll()
	bw.GetCRequestHandlerT().UnbindAll()
	bw.GetCDisplayHandlerT().UnbindAll()
	bw.resourceManager.GetCResourceRequestHandlerT().UnbindAll()
}

func (bw *BrowserWindowStd) CreateBrowser(
	initial_url string,
	parentHwnd win32api.HWND,
	rect capi.CRectT,
	settings *capi.CBrowserSettingsT,
	extra_info *capi.CDictionaryValueT,
	request_context *capi.CRequestContextT,
) {
	windowInfo := &capi.CWindowInfoT{}
	windowInfo.SetParentWindow(capi.ToCWindowHandleT(syscall.Handle(parentHwnd)))
	windowInfo.SetBounds(rect)
	windowInfo.SetStyle(win32api.WsChild | win32api.WsClipchildren | win32api.WsClipsiblings | win32api.WsTabstop | win32api.WsVisible)

	if exStyle, err := win32api.GetWindowLongPtr(parentHwnd, win32api.GwlExstyle); err == nil {
		if exStyle&win32api.WsExNoactivate != 0 {
			windowInfo.SetExStyle(windowInfo.ExStyle() | win32api.WsExNoactivate)
		}
	}

	capi.BrowserHostCreateBrowser(
		windowInfo,
		bw.GetCClientT(),
		initial_url,
		settings,
		extra_info,
		request_context,
	)
}

func (bw *BrowserWindowStd) SetFocus(focus bool) {
	if bw.GetCBrowserT() != nil {
		h := bw.GetCBrowserT().GetHost()
		h.SetFocus(focus)
		h.Unref()
	}
}

func (bw *BrowserWindowStd) Hide() {
	hwnd := GetWindowHandle(bw.GetCBrowserT())
	if hwnd != 0 {
		win32api.SetWindowPos(hwnd, 0, 0, 0, 0, 0,
			win32api.SwpNozorder|win32api.SwpNomove|win32api.SwpNoactivate)
	}

}

func (bw *BrowserWindowStd) Show() {
	hwnd := GetWindowHandle(bw.GetCBrowserT())
	if hwnd != 0 && !win32api.IsWindowVisible(hwnd) {
		win32api.ShowWindow(hwnd, win32api.SwShow)
	}
}

func GetWindowHandle(browser *capi.CBrowserT) win32api.HWND {
	if browser != nil {
		h := browser.GetHost().GetWindowHandle()
		return win32api.HWND(capi.ToHandle(h))
	}
	return 0
}

func SetBounds(browser *capi.CBrowserT, x, y int, width, height uint32) {
	if !capi.CurrentlyOn(capi.TidUi) {
		cef.PostTask(capi.TidUi, cef.TaskFunc(func() {
			SetBounds(browser, x, y, width, height)
		}))
		return
	}

	hwnd := GetWindowHandle(browser)
	if hwnd != 0 {
		win32api.SetWindowPos(hwnd, 0, x, y, int(width), int(height), win32api.SwpNozorder)
	}
}

func (bw *BrowserWindowStd) IsClosing() bool {
	return bw.is_closing_
}

func (bw *BrowserWindowStd) GetResourceRequestHandler(
	self *capi.CRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	is_navigation int,
	is_download int,
	request_initiator string,
) (*capi.CResourceRequestHandlerT, bool) {
	return bw.resourceManager.GetCResourceRequestHandlerT(), false
}

func (bw *BrowserWindowStd) OnOpenUrlfromTab(
	self *capi.CRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	target_url string,
	target_disposition capi.CWindowOpenDispositionT,
	user_gesture bool,
) (ret bool) {
	log.Println("T295: OnOpenUrlfromTab", target_disposition, user_gesture)
	switch target_disposition {
	case capi.WodNewBackgroundTab, capi.WodNewForegroundTab:
		rect := win32api.Rect{}
		browserSettings := capi.NewCBrowserSettingsT()
		config := mainConfig
		config.main_url = target_url
		config.use_windowless_rendering = bw.IsOsr()
		windowManager.CreateRootWindow(
			config, false, rect, browserSettings,
		)
		return true
	}

	return false
}

func (bw *BrowserWindowStd) OnBeforeBrowse(
	self *capi.CRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	user_gesture bool,
	is_redirect bool,
) bool {
	if !capi.CurrentlyOn(capi.TidUi) {
		log.Panicln("T360:")
	}
	if frame.IsMain() {
		router.BrowserCancelPendingForBrowser(browser)
	}

	return false
}

const routerMessagePrefix = "cef"

func (bw *BrowserWindowStd) OnProcessMessageReceived(
	self *capi.CClientT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	source_process capi.CProcessIdT,
	message *capi.CProcessMessageT,
) (ret bool) {
	log.Println("T381:")
	return router.BrowserOnProcessMessageReceived(bw, browser, frame, routerMessagePrefix, message)
}

const (
	kPrompt    = "Prompt."
	kPromptFPS = "Prompt.FPS:"
	kPromptDSF = "Prompt.DSF:"
)

func (bw *BrowserWindowStd) OnQuery(browser *capi.CBrowserT, frame *capi.CFrameT, request_str string, persistent bool, queryId router.BrowserQueryId, callback router.Callback) (handled bool) {
	return browserWindowOnQuery(bw, browser, frame, request_str, persistent, callback)
}

func browserWindowOnQuery(bw BrowserWindow, browser *capi.CBrowserT, frame *capi.CFrameT, request_str string, persistent bool, callback router.Callback) (handled bool) {
	log.Println("T396:", request_str)
	if strings.HasPrefix(request_str, kPromptFPS) {
		val := strings.TrimPrefix(request_str, kPromptFPS)
		if fps, err := strconv.Atoi(val); err == nil {
			if fps <= 0 {
				browser.GetHost().SetWindowlessFrameRate(mainConfig.windowless_frame_rate)
			} else {
				log.Println("T403:", fps)
				browser.GetHost().SetWindowlessFrameRate(fps)
			}
		}
		handled = true
	} else if strings.HasPrefix(request_str, kPromptDSF) {
		val := strings.TrimPrefix(request_str, kPromptDSF)
		if dsf, err := strconv.ParseFloat(val, 32); err == nil {
			if dsfer, ok := bw.(DeviceScaleFactorer); ok {
				dsfer.SetDeviceScaleFactor(float32(dsf))
			}
		}
		handled = true
	}
	callback.Success("")

	return handled
}

func (bw *BrowserWindowStd) OnQueryCanceled(browser *capi.CBrowserT, frame *capi.CFrameT, query_id router.BrowserQueryId) {
	// Nothing to do
}

func (rm *ResourceManager) OnBeforeResourceLoad(
	self *capi.CResourceRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
	callback *capi.CCallbackT,
) (ret capi.CReturnValueT) {
	requestUrl := request.GetUrl()
	if u, err := url.Parse(requestUrl); err == nil {
		if u.Host == kTestHost && u.Path == kTestDir+kTestRequestPage {
			stream, header_map := GetDumpResponse(request)
			rm.AddStreamResource(requestUrl, stream, header_map)
		} else {
			log.Println("T442:", u.Path)
			filterdPath := urlPathFilter(u)
			if binaryId, ok := resourceMap[strings.TrimPrefix(filterdPath, kTestDir)]; ok {
				res := LoadBinaryResource(binaryId)
				// log.Println("T445:", res)
				rm.AddBytesResource(requestUrl, "text/html", res)
			} else {
				log.Println("T449: Not exist resource", filterdPath)
			}
		}
	} else {
		log.Panicln("T441:", err, request.GetUrl())
	}
	return capi.RvContinue
}

func urlPathFilter(u *url.URL) string {
	if u.Host != kTestHost {
		return u.Path
	}
	ps := strings.Split(u.Path, "/")
	last := ps[len(ps)-1]
	if last == "" || strings.Contains(last, ".") {
		return u.Path
	}
	return u.Path + ".html"
}

const kTestHost = "tests"
const kLocalHost = "localhost"
const kTestDir = "/"
const kTestOrigin = "http://" + kTestHost + kTestDir
const kTestGetSourcePage = "get_source.html"
const kTestGetTextPage = "get_text.html"
const kTestRequestPage = "request.html"
const kTestPluginInfoPage = "plugin_info.html"

func (rm *ResourceManager) GetResourceHandler(
	self *capi.CResourceRequestHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	request *capi.CRequestT,
) (handler *capi.CResourceHandlerT) {
	if rh, ok := rm.rh[request.GetUrl()]; ok {
		handler = rh
	}
	return handler
}

type ResourceManager struct {
	rh map[string]*capi.CResourceHandlerT
	capi.RefToCResourceRequestHandlerT
}

func init() {
	// capi.CResourceRequestHandlerT
	var rrh *ResourceManager
	var _ capi.OnBeforeResourceLoadHandler = rrh
	var _ capi.GetResourceHandlerHandler = rrh
}

func NewResourceManager() (rm *ResourceManager) {
	rm = &ResourceManager{
		rh: map[string]*capi.CResourceHandlerT{},
	}
	capi.AllocCResourceRequestHandlerT().Bind(rm)
	return rm
}

func (rm *ResourceManager) AddBytesResource(url, mime string, content []byte) {
	rh := &StringResourceHandler{url, content, mime, 0}
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
			if strings.HasPrefix(value, "https://"+kTestHost) ||
				strings.HasPrefix(value, "https://"+kLocalHost) {
				capi.StringMultimapAppend(responseHeaderMap.CefObject(), "Access-Control-Allow-Origin", value)
				break
			}
		}
	}
	if n > 0 {
		capi.StringMultimapAppend(responseHeaderMap.CefObject(), "Access-Control-Allow-Headers", "My-Custom-Header")
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

func (rm *ResourceManager) AddStreamResource(url string, stream *capi.CStreamReaderT, headerMap *cef.StringMultimap) {
	rh := &StreamResourceHandler{url, stream, headerMap, "text/html", 200, "OK"}
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

func GetSource(bw BrowserWindow) {
	browser := bw.GetCBrowserT()
	rm := bw.GetResourceManager()
	url := kTestOrigin + kTestGetSourcePage
	mySv := myStringVisitor{
		f: func(c string) {
			rm.AddBytesResource(url, "text/html", []byte(c))
			browser.GetMainFrame().LoadUrl(url)
		},
	}
	sv := capi.AllocCStringVisitorT().Bind(&mySv)
	browser.GetMainFrame().GetSource(sv)
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

	sv.f(ss)
}

func GetText(bw BrowserWindow) {
	browser := bw.GetCBrowserT()
	rm := bw.GetResourceManager()
	url := kTestOrigin + kTestGetTextPage
	mySv := myStringVisitor{
		f: func(c string) {
			rm.AddBytesResource(url, "text/html", []byte(c))
			browser.GetMainFrame().LoadUrl(url)
		},
	}
	sv := capi.AllocCStringVisitorT().Bind(&mySv)
	browser.GetMainFrame().GetText(sv)
}

func (bw *BrowserWindowStd) OnAddressChange(
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
	browser *capi.CBrowserT
	rm      *ResourceManager
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

	v.html += "\n<br/><br/>Name: " + name +
		"\n<br/>Description: " + desc +
		"\n<br/>Version: " + ver +
		"\n<br/>Path: " + path
	if count+1 >= total {
		v.html += "\n</body></html>"
		url := kTestOrigin + kTestPluginInfoPage
		v.rm.AddBytesResource(url, "text/html", []byte(v.html))
		v.browser.GetMainFrame().LoadUrl(url)
	}
	return true
}

func GetPlugInInfoVisitor(browser *capi.CBrowserT, rm *ResourceManager) *capi.CWebPluginInfoVisitorT {
	visitor := &myPluginInfoVisitor{}
	visitor.html = "<html><head><title>Plugin Info Test</title></head>" +
		"<body bgcolor=\"white\">" +
		"\n<b>Installed plugins:</b>"
	visitor.browser = browser
	visitor.rm = rm

	return capi.AllocCWebPluginInfoVisitorT().Bind(visitor)
}

func (bw *BrowserWindowStd) ShowPopup(hwnd_ win32api.HWND, rect capi.CRectT) {
	bwHwnd := GetWindowHandle(bw.GetCBrowserT())
	if bwHwnd != 0 {
		if _, err := win32api.SetParent(bwHwnd, hwnd_); err != nil {
			log.Panicln("T368:", err)
		}
		if err := win32api.SetWindowPos(bwHwnd, 0,
			rect.X(), rect.Y(), rect.Width(), rect.Height(),
			win32api.SwpNozorder|win32api.SwpNoactivate); err != nil {
			log.Panicln("T372:", err)
		}
		if exStyle, err := win32api.GetWindowLongPtr(hwnd_, win32api.GwlExstyle); err == nil {
			swFlag := win32api.SwShow
			if exStyle&win32api.WsExNoactivate != 0 {
				swFlag = win32api.SwShownoactivate
			}
			win32api.ShowWindow(bwHwnd, swFlag)
		} else {
			log.Panicln("T372:", err)
		}
	}
}
