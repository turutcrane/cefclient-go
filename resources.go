package main

// #cgo pkg-config: cefingo
// #include "tests/cefclient/browser/resource.h"
import "C"
import (
	"bytes"
	"embed"
	"encoding/binary"
	"log"
	"unsafe"

	"github.com/turutcrane/win32api"
)

//go:embed resources
var resFs embed.FS

const (
	RtBinary = C.BINARY
)

const (
	IdQuit              = C.ID_QUIT
	IdFind              = C.ID_FIND
	IdTestsFirst        = C.ID_TESTS_FIRST
	IdTestsGetsource    = C.ID_TESTS_GETSOURCE
	IdTestsGettext      = C.ID_TESTS_GETTEXT
	IdTestsOtherTests   = C.ID_TESTS_OTHER_TESTS
	// IdTestsPluginInfo   = C.ID_TESTS_PLUGIN_INFO
	IdTestsWindowNew    = C.ID_TESTS_WINDOW_NEW
	IdTestsWindowPopup  = C.ID_TESTS_WINDOW_POPUP
	IdTestsPrint        = C.ID_TESTS_PRINT
	IdTestsRequest      = C.ID_TESTS_REQUEST
	IdTestsTracingBegin = C.ID_TESTS_TRACING_BEGIN
	IdTestsTracingEnd   = C.ID_TESTS_TRACING_END
	IdTestsZoomIn       = C.ID_TESTS_ZOOM_IN
	IdTestsZoomOut      = C.ID_TESTS_ZOOM_OUT
	IdTestsZoomReset    = C.ID_TESTS_ZOOM_RESET
	IdTestsOsrFps       = C.ID_TESTS_OSR_FPS
	IdTestsOsrDsf       = C.ID_TESTS_OSR_DSF
	IdTestsPrintToPdf   = C.ID_TESTS_PRINT_TO_PDF
	IdTestsMuteAudio    = C.ID_TESTS_MUTE_AUDIO
	IdTestsUnmuteAudio  = C.ID_TESTS_UNMUTE_AUDIO
	IdTestsLast         = C.ID_TESTS_LAST
)

// const (
// 	IdiCefclient = C.IDI_CEFCLIENT
// 	IdiSmall     = C.IDI_SMALL
// )
const (
	ResCefclientIcon = "resources/win/cefclient.ico"
	ResSmallIcon     = "resources/win/small.ico"
)

const (
	IdcMyicon     = C.IDC_MYICON
	IdcCefclient  = C.IDC_CEFCLIENT
	IdcNavBack    = C.IDC_NAV_BACK
	IdcNavForward = C.IDC_NAV_FORWARD
	IdcNavReload  = C.IDC_NAV_RELOAD
	IdcNavStop    = C.IDC_NAV_STOP
	IdcStatic     = C.IDC_STATIC
)

const (
	IdmAbout = C.IDM_ABOUT
	IdmExit  = C.IDM_EXIT
)

const (
	IddCefclientDialog = C.IDD_CEFCLIENT_DIALOG
	IddAboutbox        = C.IDD_ABOUTBOX
)

const (
	IdsAppTitle      = C.IDS_APP_TITLE
	IdsBindingHtml   = C.IDS_BINDING_HTML
	IdsDialogsHtml   = C.IDS_DIALOGS_HTML
	IdsDraggableHtml = C.IDS_DRAGGABLE_HTML
	// IdsDrmHtml                            = C.IDS_DRM_HTML
	IdsLocalstorageHtml                   = C.IDS_LOCALSTORAGE_HTML
	IdsLogoPng                            = C.IDS_LOGO_PNG
	IdsMediaRouterHtml                    = C.IDS_MEDIA_ROUTER_HTML
	IdsMenuIcon1xPng                      = C.IDS_MENU_ICON_1X_PNG
	IdsMenuIcon2xPng                      = C.IDS_MENU_ICON_2X_PNG
	IdsOsrtestHtml                        = C.IDS_OSRTEST_HTML
	IdsOtherTestsHtml                     = C.IDS_OTHER_TESTS_HTML
	IdsPdfHtml                            = C.IDS_PDF_HTML
	IdsPdfPdf                             = C.IDS_PDF_PDF
	IdsPerformanceHtml                    = C.IDS_PERFORMANCE_HTML
	IdsPerformance2Html                   = C.IDS_PERFORMANCE2_HTML
	IdsPreferencesHtml                    = C.IDS_PREFERENCES_HTML
	IdsResponseFilterHtml                 = C.IDS_RESPONSE_FILTER_HTML
	IdsServerHtml                         = C.IDS_SERVER_HTML
	IdsTransparencyHtml                   = C.IDS_TRANSPARENCY_HTML
	IdsUrlrequestHtml                     = C.IDS_URLREQUEST_HTML
	IdsWebsocketHtml                      = C.IDS_WEBSOCKET_HTML
	IdsWindowHtml                         = C.IDS_WINDOW_HTML
	IdsWindowIcon1xPng                    = C.IDS_WINDOW_ICON_1X_PNG
	IdsWindowIcon2xPng                    = C.IDS_WINDOW_ICON_2X_PNG
	IdsXmlhttprequestHtml                 = C.IDS_XMLHTTPREQUEST_HTML
	IdsExtensionsSetPageColorIconPng      = C.IDS_EXTENSIONS_SET_PAGE_COLOR_ICON_PNG
	IdsExtensionsSetPageColorManifestJson = C.IDS_EXTENSIONS_SET_PAGE_COLOR_MANIFEST_JSON
	IdsExtensionsSetPageColorPopupHtml    = C.IDS_EXTENSIONS_SET_PAGE_COLOR_POPUP_HTML
	IdsExtensionsSetPageColorPopupJs      = C.IDS_EXTENSIONS_SET_PAGE_COLOR_POPUP_JS
)

var resourceMap = map[string]int{
	"binding.html":                            IdsBindingHtml,
	"dialogs.html":                            IdsDialogsHtml,
	"draggable.html":                          IdsDraggableHtml,
	"extensions/set_page_color/icon.png":      IdsExtensionsSetPageColorIconPng,
	"extensions/set_page_color/manifest.json": IdsExtensionsSetPageColorManifestJson,
	"extensions/set_page_color/popup.html":    IdsExtensionsSetPageColorPopupHtml,
	"extensions/set_page_color/popup.js":      IdsExtensionsSetPageColorPopupJs,
	"localstorage.html":                       IdsLocalstorageHtml,
	"logo.png":                                IdsLogoPng,
	"media_router.html":                       IdsMediaRouterHtml,
	"menu_icon.1x.png":                        IdsMenuIcon1xPng,
	"menu_icon.2x.png":                        IdsMenuIcon2xPng,
	"osr_test.html":                           IdsOsrtestHtml,
	"other_tests.html":                        IdsOtherTestsHtml,
	"pdf.html":                                IdsPdfHtml,
	"pdf.pdf":                                 IdsPdfPdf,
	"performance.html":                        IdsPerformanceHtml,
	"performance2.html":                       IdsPerformance2Html,
	"preferences.html":                        IdsPreferencesHtml,
	"response_filter.html":                    IdsResponseFilterHtml,
	"server.html":                             IdsServerHtml,
	"transparency.html":                       IdsTransparencyHtml,
	"urlrequest.html":                         IdsUrlrequestHtml,
	"websocket.html":                          IdsWebsocketHtml,
	"window.html":                             IdsWindowHtml,
	"window_icon.1x.png":                      IdsWindowIcon1xPng,
	"window_icon.2x.png":                      IdsWindowIcon2xPng,
	"xmlhttprequest.html":                     IdsXmlhttprequestHtml,
}

func LoadBinaryResource(binaryId int) []byte {

	hInst, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T445:", err)
	}
	hRes, err := win32api.FindResource(hInst, win32api.MakeIntResource(uint16(binaryId)), win32api.MakeIntResource(RtBinary))
	if err != nil {
		log.Panicln("T449:", err)
	}
	hGlob, err := win32api.LoadResource(hInst, hRes)
	if err != nil {
		log.Panicln("T453:", err)
	}
	size, err := win32api.SizeofResource(hInst, hRes)
	if err != nil {
		log.Panicln("T147:", err)
	}
	p, err := win32api.LockResource(hGlob)
	if err != nil {
		log.Panicln("T457:", err)
	}
	return C.GoBytes(unsafe.Pointer(uintptr(p)), C.int(size))
}

type IconDir struct {
	Reserved      uint16
	ResourceType  uint16
	ResourceCount uint16
}

type IconDirEntry struct {
	Width        byte
	Height       byte
	ColorCount   byte
	Rsrvd1       byte
	ColorPlane   uint16
	PixelPerBits uint16
	DataCount    uint32
	DataOffset   uint32
}

func loadIconResource(handle win32api.HINSTANCE, resPath string) (icon win32api.HICON) {
	// icBytes, err := resFs.ReadFile(ResSmallIcon)
	// icBytes, err := resFs.ReadFile(ResCefclientIcon)
	icBytes, err := resFs.ReadFile(resPath)
	if err != nil {
		log.Panicln("T164:", err)
	}
	b := bytes.NewReader(icBytes)

	var dir IconDir
	if err := binary.Read(b, binary.LittleEndian, &dir); err != nil {
		log.Panicln("T42:", err)
	}
	// log.Println("T45:", dir)

	inum := int(dir.ResourceCount)
	var icons []IconDirEntry
	// offset := dirOffset + i*16
	for i := 0; i < inum; i++ {
		var entry IconDirEntry
		if err := binary.Read(b, binary.LittleEndian, &entry); err != nil {
			log.Panicln("T44:", err)
		}
		icons = append(icons, entry)
		// fmt.Println("T56:", entry)
	}

	xpixel := win32api.GetSystemMetrics(win32api.SmCxicon)
	ypixel := win32api.GetSystemMetrics(win32api.SmCyicon)
	screen_dc := win32api.GetDC(0)
	bitsPixel := win32api.GetDeviceCaps(screen_dc, win32api.Bitspixel)
	planes := win32api.GetDeviceCaps(screen_dc, win32api.Planes)
	// fmt.Println("T64:", planes, bitsPixel)

	for _, ic := range icons {
		b = bytes.NewReader(icBytes[ic.DataOffset:])
		var bitMap win32api.Bitmapinfoheader
		if err := binary.Read(b, binary.LittleEndian, &bitMap); err != nil {
			log.Panicln("T59:", err)
		}
		if bitMap.Size != win32api.DWORD(unsafe.Sizeof(bitMap)) {
			log.Panicln("T73: Size mismatch", bitMap.Size)
		}
		if int(ic.Width) == xpixel && int(ic.Height) == ypixel &&
			int(bitMap.Planes) == planes && int(bitMap.BitCount) == bitsPixel {
			// log.Println("T52:", ic)
			// log.Println("T68:", bitMap)
			if bitMap.Compression != win32api.BiRgb {
				log.Panicln("T77: Not suported type", bitMap.Compression)
			}
			if bitMap.BitCount <= 8 {
				log.Panicln("T80: Not suported type", bitMap.BitCount)
			}
			colorTableSize := 0
			pixelDataSize := int(ic.Width) * int(ic.Height) * (32 / 8)
			pixelDataOffset := int(ic.DataOffset) + int(bitMap.Size) + colorTableSize
			maskDataSize := (((3 + int(ic.Width)/8) / 4) * 4) * int(ic.Height)
			maskDataOffset := pixelDataOffset + pixelDataSize

			if int(unsafe.Sizeof(bitMap))+colorTableSize+pixelDataSize+maskDataSize != int(ic.DataCount) {
				log.Panicln("T90: Not suported type", ic.Width, ic.Height, pixelDataSize, maskDataSize, ic, bitMap)
			}
			pixelP := (*win32api.BYTE)(unsafe.Pointer(&icBytes[pixelDataOffset]))
			maskP := (*win32api.BYTE)(unsafe.Pointer(&icBytes[maskDataOffset]))
			icon, err := win32api.CreateIcon(
				handle,
				int(ic.Width), int(ic.Height),
				win32api.BYTE(planes),
				win32api.BYTE(bitsPixel),
				maskP, pixelP,
			)
			if err != nil {
				log.Panicln("T246:", err)
			}
			return icon
		}
	}

	return
}
