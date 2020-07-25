package main

// #cgo pkg-config: cefingo
// #include "tests/cefclient/browser/resource.h"
import "C"

const (
	IdQuit              = C.ID_QUIT
	IdFind              = C.ID_FIND
	IdTestsFirst        = C.ID_TESTS_FIRST
	IdTestsGetsource    = C.ID_TESTS_GETSOURCE
	IdTestsGettext      = C.ID_TESTS_GETTEXT
	IdTestsOtherTests   = C.ID_TESTS_OTHER_TESTS
	IdTestsPluginInfo   = C.ID_TESTS_PLUGIN_INFO
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

const (
	IdiCefclient = C.IDI_CEFCLIENT
	IdiSmall     = C.IDI_SMALL
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
