package main

import (
	"log"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/win32api"
)

type WindowManager struct {
	rootWindowMap sync.Map // hwnd -> *RootWindowWin

	rootWinsLock sync.Mutex
	rootWins     []*RootWindowWin

	active_root_window_  *RootWindowWin
	active_browser_lock_ sync.Mutex
	active_browser_      *capi.CBrowserT

	temp_window_ win32api.HWND

	browserWindowOsrMap sync.Map // hwnd -> *BrowserWindowOsrMap
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
	wndClass.Instance = win32api.HINSTANCE(hInstance)
	wndClass.ClassName = syscall.StringToUTF16Ptr(kWndClass)
	if _, err := win32api.RegisterClassEx(&wndClass); err != nil {
		log.Panicln("T60: Can not Register Class", kWndClass, err)
	}
	if wm.temp_window_, err = win32api.CreateWindowEx(0,
		syscall.StringToUTF16Ptr(kWndClass), nil,
		win32api.WsOverlappedwindow|win32api.WsClipchildren,
		0, 0, 1, 1, 0, 0, win32api.HINSTANCE(hInstance), 0,
	); err != nil {
		log.Panicln("T69: Failed to CreateWindowsEx", err)
	}
	return wm.temp_window_
}

func (wm *WindowManager) CreateRootWindow(
	config ClientConfig,
	is_popup bool,
	rect win32api.Rect,
	browserSettings *capi.CBrowserSettingsT,
) (rootWindow *RootWindowWin) {
	rootWindow = wm.NewRootWindowWin()
	rootWindow.Init(config, is_popup, rect, browserSettings)

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
	wm.rootWindowMap.Store(hwnd, rootWin)
}

func (wm *WindowManager) GetRootWin(hwnd win32api.HWND) (rootWin *RootWindowWin, exist bool) {
	if r, ok := wm.rootWindowMap.Load(hwnd); ok {
		rootWin = r.(*RootWindowWin)
		exist = ok
	}
	return rootWin, exist
}

func (wm *WindowManager) GetRootWinForBrowser(browserId int) (rootWin *RootWindowWin) {
	wm.rootWindowMap.Range(func(key interface{}, value interface{}) bool {
		rw := value.(*RootWindowWin)
		if rw.GetBrowser().GetIdentifier() == browserId {
			rootWin = rw
			return false
		}
		return true
	})
	return rootWin
}

func (wm *WindowManager) RemoveRootWin(hwnd win32api.HWND) {
	wm.rootWindowMap.Delete(hwnd)
}

func (wm *WindowManager) Empty() bool {
	var mapEmpty = true
	wm.rootWindowMap.Range(func(key, value interface{}) bool {
		mapEmpty = false
		return false
	})
	return mapEmpty
}

func (wm *WindowManager) CloseAllWindows(force bool) {
	wm.rootWindowMap.Range(func(key, value interface{}) bool {
		rw := value.(*RootWindowWin)
		rw.Close(force)

		return true
	})
	// wm.Map = sync.Map{}
	// wm.rootWins = nil
	// wm.temp_window_ = 0
}

func (wm *WindowManager) SetBrowserWindowOsr(hwnd win32api.HWND, browserWindowOsr *BrowserWindowOsr) {
	wm.browserWindowOsrMap.Store(hwnd, browserWindowOsr)
}

func (wm *WindowManager) GetBrowserWindowOsr(hwnd win32api.HWND) (browserWindowOsr *BrowserWindowOsr, exist bool) {
	if r, ok := wm.browserWindowOsrMap.Load(hwnd); ok {
		browserWindowOsr = r.(*BrowserWindowOsr)
		exist = ok
	}
	return browserWindowOsr, exist
}

func (wm *WindowManager) RemoveBrowserWindowOsr(hwnd win32api.HWND) {
	wm.browserWindowOsrMap.Delete(hwnd)
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

var class_regsitered bool

func RegisterRootClass(hInstance win32api.HINSTANCE, window_class string, background_brush win32api.HBRUSH) {
	if class_regsitered {
		return
	}
	class_regsitered = true

	// icon, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(IdiCefclient))
	// if err != nil {
	// 	log.Panicln("T105: LoadIcon", err)
	// }
	icon := loadIconResource(hInstance, ResCefclientIcon)
	// iconSm, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(IdiSmall))
	// if err != nil {
	// 	log.Panicln("T109: LoadIcon Sm", err)
	// }
	iconSm := loadIconResource(hInstance, ResSmallIcon)
	cursor, err := win32api.LoadCursor(0, win32api.MakeIntResource(win32api.IdcArrow))
	if err != nil {
		log.Panicln("T113: LoadCursor", err)
	}
	wndClass := win32api.Wndclassex{
		Size:       win32api.UINT(unsafe.Sizeof(win32api.Wndclassex{})),
		Style:      win32api.CsHredraw | win32api.CsVredraw,
		WndProc:    win32api.WNDPROC(syscall.NewCallback(RootWndProc)),
		ClsExtra:   0,
		WndExtra:   0,
		Instance:   hInstance,
		Icon:       icon,
		Cursor:     cursor,
		Background: background_brush,
		MenuName:   win32api.MakeIntResource(IdcCefclient),
		ClassName:  syscall.StringToUTF16Ptr(window_class),
		IconSm:     iconSm,
	}
	if _, err := win32api.RegisterClassEx(&wndClass); err != nil {
		log.Panicln("T109: Can not Register Class", window_class, err)
	}
	return
}

var class_regsitered_osr bool

func RegisterOsrClass(hInstance win32api.HINSTANCE, window_class string, background_brush win32api.HBRUSH) {
	if class_regsitered_osr {
		return
	}
	class_regsitered_osr = true

	cursor, err := win32api.LoadCursor(0, win32api.MakeIntResource(win32api.IdcArrow))
	if err != nil {
		log.Panicln("T113: LoadCursor", err)
	}
	// iconSm, err := win32api.LoadIcon(hInstance, win32api.MakeIntResource(IdiSmall))
	// if err != nil {
	// 	log.Panicln("T109: LoadIcon Sm", err)
	// }
	iconSm := loadIconResource(hInstance, ResSmallIcon)
	wndClass := win32api.Wndclassex{
		Size:       win32api.UINT(unsafe.Sizeof(win32api.Wndclassex{})),
		Style:      win32api.CsOwndc,
		WndProc:    win32api.WNDPROC(syscall.NewCallback(OsrWndProc)),
		ClsExtra:   0,
		WndExtra:   0,
		Instance:   hInstance,
		Icon:       0,
		Cursor:     cursor,
		Background: background_brush,
		MenuName:   win32api.MakeIntResource(IdcCefclient),
		ClassName:  syscall.StringToUTF16Ptr(window_class),
		IconSm:     iconSm,
	}
	if _, err := win32api.RegisterClassEx(&wndClass); err != nil {
		log.Panicln("T109: Can not Register Class", window_class, err)
	}
}

var processPerMonitorDpiAware *bool

func IsProcessPerMonitorDpiAware() bool {
	if processPerMonitorDpiAware == nil {
		var dpiAwareness win32api.ProcessDpiAwareness
		var aBool bool
		processPerMonitorDpiAware = &aBool

		hresult := win32api.GetProcessDpiAwareness(0, &dpiAwareness)
		if hresult == win32api.SOk {
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
		dpi := win32api.GetDpiForWindow(hwnd)
		return float32(dpi) / DPI_1X
	}

	return GetDeviceScaleFactor()
}

func LogicalToDevice(value int, device_scale_factor float32) int {
	scaled_val := float32(value) * device_scale_factor
	return int(math.Floor(float64(scaled_val)))
}

func DeviceToLogical(value int, device_scale_factor float32) int {
	scaled_val := float32(value) / device_scale_factor
	return int(math.Floor(float64(scaled_val)))
}

func DeviceToLogicalMouseEvent(event capi.CMouseEventT, device_scale_factor float32) capi.CMouseEventT {
	event.SetX(DeviceToLogical(event.X(), device_scale_factor))
	event.SetY(DeviceToLogical(event.Y(), device_scale_factor))
	return event
}

var scale_factor float32 = 1.0
var initialized bool = false

func GetDeviceScaleFactor() float32 {

	if !initialized {
		// This value is safe to cache for the life time of the app since the user
		// must logout to change the DPI setting. This value also applies to all
		// screens.
		screen_dc := win32api.GetDC(0)
		dpi_x := win32api.GetDeviceCaps(screen_dc, win32api.Logpixelsx)
		scale_factor = float32(dpi_x) / DPI_1X
		win32api.ReleaseDC(0, screen_dc)
		initialized = true
	}

	return scale_factor
}

func SetWndProc(hWnd win32api.HWND, wndProc win32api.WndProc) win32api.WNDPROC {
	proc := syscall.NewCallback(wndProc)
	v, err := win32api.SetWindowLongPtr(hWnd, win32api.GwlpWndproc, proc)
	if err != nil {
		log.Panicln("T383:", err)
	}
	return win32api.WNDPROC(v)
}

func IsKeyDown(wparam int) bool {
	return (uint32(win32api.GetKeyState(wparam)) & 0x8000) != 0
}

func GetCefMouseModifiers(wparam win32api.WPARAM) (modifiers capi.CEventFlagsT) {
	if (wparam & win32api.MkControl) != 0 {
		modifiers |= capi.EventflagControlDown
	}
	if (wparam & win32api.MkShift) != 0 {
		modifiers |= capi.EventflagShiftDown
	}
	if IsKeyDown(win32api.VkMenu) {
		modifiers |= capi.EventflagAltDown
	}
	if (wparam & win32api.MkLbutton) != 0 {
		modifiers |= capi.EventflagLeftMouseButton
	}
	if (wparam & win32api.MkMbutton) != 0 {
		modifiers |= capi.EventflagMiddleMouseButton
	}
	if (wparam & win32api.MkRbutton) != 0 {
		modifiers |= capi.EventflagRightMouseButton
	}

	// Low bit set from GetKeyState indicates "toggled".
	if (win32api.GetKeyState(win32api.VkNumlock) & 1) != 0 {
		modifiers |= capi.EventflagNumLockOn
	}
	if (win32api.GetKeyState(win32api.VkCapital) & 1) != 0 {
		modifiers |= capi.EventflagCapsLockOn
	}
	return modifiers
}

func GetCefKeyboardModifiers(wparam win32api.WPARAM, lparam win32api.LPARAM) (modifiers capi.CEventFlagsT) {
	if IsKeyDown(win32api.VkShift) {
		modifiers |= capi.EventflagShiftDown
	}
	if IsKeyDown(win32api.VkControl) {
		modifiers |= capi.EventflagControlDown
	}
	if IsKeyDown(win32api.VkMenu) {
		modifiers |= capi.EventflagAltDown
	}

	// Low bit set from GetKeyState indicates "toggled".
	if (win32api.GetKeyState(win32api.VkNumlock) & 1) != 0 {
		modifiers |= capi.EventflagNumLockOn
	}
	if (win32api.GetKeyState(win32api.VkCapital) & 1) != 0 {
		modifiers |= capi.EventflagCapsLockOn
	}

	switch wparam {
	case win32api.VkReturn:
		if ((lparam >> 16) & win32api.KfExtended) != 0 {
			modifiers |= capi.EventflagIsKeyPad
		}

	case win32api.VkInsert, win32api.VkDelete, win32api.VkHome, win32api.VkEnd,
		win32api.VkPrior, win32api.VkNext, win32api.VkUp, win32api.VkDown, win32api.VkLeft, win32api.VkRight:
		if ((lparam >> 16) & win32api.KfExtended) == 0 {
			modifiers |= capi.EventflagIsKeyPad
		}

	case win32api.VkNumlock, win32api.VkNumpad0, win32api.VkNumpad1, win32api.VkNumpad2, win32api.VkNumpad3,
		win32api.VkNumpad4, win32api.VkNumpad5, win32api.VkNumpad6, win32api.VkNumpad7,
		win32api.VkNumpad8, win32api.VkNumpad9,
		win32api.VkDivide, win32api.VkMultiply, win32api.VkSubtract, win32api.VkAdd,
		win32api.VkDecimal, win32api.VkClear:
		modifiers |= capi.EventflagIsKeyPad

	case win32api.VkShift:
		if IsKeyDown(win32api.VkLshift) {
			modifiers |= capi.EventflagIsLeft
		} else if IsKeyDown(win32api.VkRshift) {
			modifiers |= capi.EventflagIsRight
		}

	case win32api.VkControl:
		if IsKeyDown(win32api.VkLcontrol) {
			modifiers |= capi.EventflagIsLeft
		} else if IsKeyDown(win32api.VkRcontrol) {
			modifiers |= capi.EventflagIsRight
		}

	case win32api.VkMenu:
		if IsKeyDown(win32api.VkLmenu) {
			modifiers |= capi.EventflagIsLeft
		} else if IsKeyDown(win32api.VkRmenu) {
			modifiers |= capi.EventflagIsRight
		}

	case win32api.VkLwin:
		modifiers |= capi.EventflagIsLeft

	case win32api.VkRwin:
		modifiers |= capi.EventflagIsRight
	}
	return modifiers
}

var qi_freq win32api.LARGE_INTEGER

func GetTimeNow() uint64 {
	if qi_freq == 0 {
		win32api.QueryPerformanceFrequency(&qi_freq)
	}
	var t win32api.LARGE_INTEGER
	win32api.QueryPerformanceCounter(&t)
	return uint64(float64(t) / float64(qi_freq) * 1000000)
}

func GetDownloadPath(fileName string) (path string) {
	if hresult, p := win32api.SHGetKnownFolderPath(
		win32api.FolderidDownloads,
		win32api.CsidlPersonal|win32api.CsidlFlagCreate, 0); hresult == win32api.SOk {
		path = p + "\\" + fileName
	}
	return path
}
