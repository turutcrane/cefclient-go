package main

import (
	"log"
	"math"
	"sync"
	"syscall"
	"unsafe"

	// "github.com/JamesHovious/w32"
	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
	"github.com/turutcrane/win32api/win32const"
)

type WindowManager struct {
	sync.Map // hwnd -> *RootWindowWin

	rootWinsLock sync.Mutex
	rootWins     []*RootWindowWin

	active_root_window_  *RootWindowWin
	active_browser_lock_ sync.Mutex
	active_browser_      *capi.CBrowserT

	temp_window_ win32api.HWND
}

// var rootWins = []*RootWindowWin{nil} // index 0 is not usable.

var windowManager = &WindowManager{}

func (wm *WindowManager) NewRootWindowWin() (rootWindow *RootWindowWin) {
	rootWindow = &RootWindowWin{}

	wm.rootWinsLock.Lock()
	defer wm.rootWinsLock.Unlock()
	if len(wm.rootWins) == 0 {
		wm.rootWins = append(wm.rootWins, nil)
	}
	wm.rootWins = append(wm.rootWins, rootWindow)
	rootWindow.key = len(wm.rootWins) - 1

	return rootWindow
}

func (wm *WindowManager) GetTempWindow() win32api.HWND {
	kWndClass := "Client_TempWindow"
	if wm.temp_window_ != 0 {
		return wm.temp_window_
	}
	hInstance, err := win32api.GetModuleHandle(nil)
	if err != nil {
		log.Panicln("T52:", err)
	}
	wndClass := win32api.Wndclassex{}
	wndClass.Size = win32api.UINT(unsafe.Sizeof(win32api.Wndclassex{}))
	wndClass.WndProc = win32api.WndProcToWNDPROC(win32api.DefWindowProc)
	wndClass.Instance = win32api.HINSTANCE(syscall.Handle(hInstance))
	wndClass.ClassName = syscall.StringToUTF16Ptr(kWndClass)
	if win32api.RegisterClassEx(&wndClass) == 0 {
		log.Panicln("T60: Can not Register Class", kWndClass)
	}
	hwnd, err := win32api.CreateWindowEx(0,
		syscall.StringToUTF16Ptr(kWndClass), nil,
		win32const.WsOverlappedwindow|win32const.WsClipchildren,
		0, 0, 1, 1, 0, 0, win32api.HINSTANCE(syscall.Handle(hInstance)), 0,
	)
	if err != nil {
		log.Panicln("T69: Failed to CreateWindowsEx", err)
	}
	wm.temp_window_ = hwnd
	return wm.temp_window_
}

func (wm *WindowManager) CreateRootWindow(
	inital_url string,
	is_popup bool,
	with_controls bool,
	rect win32api.Rect,
	always_on_top bool,
	no_activate bool,
	browserSettings *capi.CBrowserSettingsT,
) (rootWindow *RootWindowWin) {
	rootWindow = wm.NewRootWindowWin()
	rootWindow.Init(inital_url, is_popup, with_controls, rect, always_on_top, no_activate, browserSettings)

	if !is_popup {
		rootWindow.CreateWindow()
	}

	wm.OnRootWindowActivated(rootWindow)

	return rootWindow
}

func (wm *WindowManager) Lookup(key int) (rootWindow *RootWindowWin) {
	wm.rootWinsLock.Lock()
	defer wm.rootWinsLock.Unlock()
	return wm.rootWins[key]
}

func (wm *WindowManager) SetRootWin(hwnd win32api.HWND, rootWin *RootWindowWin) {
	wm.Store(hwnd, rootWin)
}

func (wm *WindowManager) GetRootWin(hwnd win32api.HWND) (rootWin *RootWindowWin, exist bool) {
	if r, ok := wm.Load(hwnd); ok {
		rootWin = r.(*RootWindowWin)
		exist = ok
	}
	return rootWin, exist
}

func (wm *WindowManager) RemoveRootWin(hwnd win32api.HWND) {
	wm.Delete(hwnd)
}

func (wm *WindowManager) Empty() bool {
	var mapEmpty = true
	wm.Range(func(key, value interface{}) bool {
		mapEmpty = false
		return false
	})
	return mapEmpty
}

func (wm *WindowManager) CloseAllWindows(force bool) {
	wm.Range(func(key, value interface{}) bool {
		rw := value.(*RootWindowWin)
		rw.Close(force)

		return true
	})
	// wm.Map = sync.Map{}
	// wm.rootWins = nil
	// wm.temp_window_ = 0
}

func (wm *WindowManager) GetActiveBrowser() *capi.CBrowserT {
	wm.active_browser_lock_.Lock()
	defer wm.active_browser_lock_.Unlock()
	return wm.active_browser_
}

func (wm *WindowManager) OnRootWindowActivated(root_window *RootWindowWin) {
	if root_window.WithExtesion() {
		// We don't want extension apps to become the active RootWindow.
		return
	}
	if root_window == wm.active_root_window_ {
		return
	}
	wm.active_root_window_ = root_window

	wm.active_browser_lock_.Lock()
	defer wm.active_browser_lock_.Unlock()
	wm.active_browser_ = wm.active_root_window_.GetBrowser()
}

func (wm *WindowManager) OnRootWindowDestroyed(root_window *RootWindowWin) {
	// log.Println("T118:", "OnBeforeClose: QuitMessageLoop")

	if root_window.edit_hwnd_ != 0 {
		wm.RemoveRootWin(root_window.edit_hwnd_)
	}
	wm.RemoveRootWin(root_window.hwnd_)

	if wm.active_root_window_ == root_window {
		wm.active_root_window_ = nil

		wm.active_browser_lock_.Lock()
		wm.active_browser_ = nil
		wm.active_browser_lock_.Unlock()
	}

	if wm.Empty() {
		capi.QuitMessageLoop()
	}
}

func (wm *WindowManager) OnBrowserCreated(root_window *RootWindowWin, browser *capi.CBrowserT) {
	if root_window == wm.active_root_window_ {
		wm.active_browser_lock_.Lock()
		defer wm.active_browser_lock_.Unlock()
		wm.active_browser_ = browser
	}
}

const (
	MAX_URL_LENGTH = 255
	BUTTON_WIDTH   = 72
	URLBAR_HEIGHT  = 24
)

func RGB(r, g, b uint32) win32api.COLORREF {
	return win32api.COLORREF(r | g<<8 | b<<16)
}

func CefColorSetARGB(a, r, g, b int) capi.CColorT {
	return capi.CColorT(a<<24 | r<<16 | g<<8 | b)
}

func CefColorGetA(c capi.CColorT) uint32 {
	return (uint32(c) >> 24) & 0xff
}

func CefColorGetR(c capi.CColorT) uint32 {
	return (uint32(c) >> 16) & 0xff
}

func CefColorGetG(c capi.CColorT) uint32 {
	return (uint32(c) >> 8) & 0xff
}

func CefColorGetB(c capi.CColorT) uint32 {
	return uint32(c) & 0xff
}

var class_regsitered bool

func RegisterRootClass(hInstance win32api.HINSTANCE, window_class string, background_brush win32api.HBRUSH) {
	if class_regsitered {
		return
	}
	class_regsitered = true

	icon, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(IdiCefclient)) // w32.IDI_APPLICATION
	if err != nil {
		log.Panicln("T105: LoadIcon", err)
	}
	// icon := win32api.HICON(w32.LoadIcon(w32.HINSTANCE(hInstance), w32.MakeIntResource(C.IDI_CEFCLIENT))) // w32.IDI_APPLICATION
	// log.Panicln("T114: LoadIcon", icon, w32.GetLastError())

	iconSm, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(IdiSmall))
	if err != nil {
		log.Panicln("T109: LoadIcon Sm", err)
	}
	cursor, err := win32api.LoadCursor(0, win32const.IdcArrow)
	if err != nil {
		log.Panicln("T113: LoadCursor", err)
	}
	wndClass := win32api.Wndclassex{
		Size:       win32api.UINT(unsafe.Sizeof(win32api.Wndclassex{})),
		Style:      win32const.CsHredraw | win32const.CsVredraw,
		WndProc:    win32api.WNDPROC(syscall.NewCallback(RootWndProc)),
		ClsExtra:   0,
		WndExtra:   0,
		Instance:   hInstance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 0,
		MenuName:   win32api.MakeIntResource(IdcCefclient),
		ClassName:  syscall.StringToUTF16Ptr(window_class),
		IconSm:     iconSm,
	}
	if win32api.RegisterClassEx(&wndClass) == 0 {
		log.Panicln("T109: Can not Register Class", window_class)
	}
}

var processPerMonitorDpiAware *bool

func IsProcessPerMonitorDpiAware() bool {
	if processPerMonitorDpiAware == nil {
		var dpiAwareness win32api.ProcessDpiAwareness
		var aBool bool
		processPerMonitorDpiAware = &aBool

		hresult := win32api.GetProcessDpiAwareness(0, &dpiAwareness)
		if hresult == win32const.SOk {
			*processPerMonitorDpiAware = true
		}
	}
	return *processPerMonitorDpiAware
}

func GetButtonWidth(hwnd win32api.HWND) int {
	return LogicalToDevice(BUTTON_WIDTH, GetWindowScaleFactor(hwnd))
}

func GetURLBarHeight(hwnd win32api.HWND) int {
	return LogicalToDevice(URLBAR_HEIGHT, GetWindowScaleFactor(hwnd))
}

const (
	DPI_1X = 96.0
)

func GetWindowScaleFactor(hwnd win32api.HWND) float32 {
	if hwnd != 0 && IsProcessPerMonitorDpiAware() {
		return float32(win32api.GetDpiForWindow(hwnd)) / DPI_1X
	}
	return GetDeviceScaleFactor()
}

func LogicalToDevice(value int, device_scale_factor float32) int {
	scaled_val := float32(value) * device_scale_factor
	return int(math.Floor(float64(scaled_val)))
}

var scale_factor float32 = 1.0
var initialized bool = false

func GetDeviceScaleFactor() float32 {

	if !initialized {
		// This value is safe to cache for the life time of the app since the user
		// must logout to change the DPI setting. This value also applies to all
		// screens.
		screen_dc := win32api.GetDC(0)
		dpi_x := win32api.GetDeviceCaps(screen_dc, win32const.Logpixelsx)
		scale_factor = float32(dpi_x) / DPI_1X
		win32api.ReleaseDC(0, screen_dc)
		initialized = true
	}

	return scale_factor
}

func SetWndProc(hWnd win32api.HWND, wndProc win32api.WndProc) win32api.WNDPROC {
	proc := syscall.NewCallback(wndProc)
	v, err := win32api.SetWindowLongPtr(hWnd, win32const.GwlpWndproc, proc)
	if err != nil {
		log.Panicln("T383:", err)
	}
	return win32api.WNDPROC(v)
}
