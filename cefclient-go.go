package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
	"github.com/turutcrane/win32api"
)

type ClientConfig struct {
	main_url                     string
	use_windowless_rendering     bool
	use_transparent_painting     bool
	external_begin_frame_enabled bool
	windowless_frame_rate        int
	background_color             capi.CColorT

	// RootWindowConfig
	always_on_top bool
	with_controls bool

	no_activate bool
}

var mainConfig ClientConfig

func init() {
	// prefix := fmt.Sprintf("[%d] ", os.Getpid())
	// capi.Logger = log.New(os.Stdout, prefix, log.LstdFlags)
	// capi.RefCountLogOutput(true)
}

func main() {
	// capi.Initialize(i.e. cef_initialize) and some function should be called on
	// the main application thread to initialize the CEF browser process
	runtime.LockOSThread()
	go func() {
		ppid := os.Getppid()
		proc, _ := os.FindProcess(ppid)
		status, _ := proc.Wait()
		log.Println("Parent:", ppid, status)
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}()

	// log.Println("T38:", os.Getpid(), os.Args)
	capi.EnableHighdpiSupport()

	mainArgs := capi.NewCMainArgsT()
	cef.CMainArgsTSetInstance(mainArgs)

	app := &myApp{}
	capi.AllocCAppT().Bind(app)
	defer app.GetCAppT().UnbindAll()

	capi.AllocCBrowserProcessHandlerT().Bind(app)
	defer app.GetCBrowserProcessHandlerT().UnbindAll()

	cef.ExecuteProcess(mainArgs, app.GetCAppT())

	// browser_process_handler.initial_url = flag.String("url", "https://www.golang.org/", "URL")
	flag.StringVar(&mainConfig.main_url, "url", "https://www.golang.org/", "URL")
	flag.BoolVar(&mainConfig.use_windowless_rendering, "osr", false, "with Off Screen Rendering")
	flag.BoolVar(&mainConfig.always_on_top, "always-on-top", false, "always-on-top")
	flag.BoolVar(&mainConfig.no_activate, "no-activate", false, "no-ctivate")
	flag.BoolVar(&mainConfig.with_controls, "with-controls", true, "invert hide-controls")
	flag.Parse() // should be after cef.ExecuteProcess() or implement CComandLine

	log.Println("T74:", mainConfig.with_controls)
	
	s := capi.NewCSettingsT()
	s.SetLogSeverity(capi.LogseverityWarning)
	s.SetNoSandbox(true)
	s.SetMultiThreadedMessageLoop(false)
	s.SetRemoteDebuggingPort(8088)

	cef.Initialize(mainArgs, s, app.GetCAppT())
	runtime.UnlockOSThread()

	browserSettings := capi.NewCBrowserSettingsT()
	rect := win32api.Rect{Left: 0, Top: 0, Right: 0, Bottom: 0}
	windowManager.CreateRootWindow(mainConfig, false, rect, browserSettings)

	capi.RunMessageLoop()
	defer capi.Shutdown()
}

type myBrowserProcessHandler struct {
	// this reference forms an UNgabagecollectable circular reference
	// To GC, call myBrowserProcessHandler.SetCBrowserProcessHandlerT(nil)
	capi.RefToCBrowserProcessHandlerT

	// capi.RefToCClientT
	// initial_url *string
}

type myApp struct {
	capi.RefToCAppT
	myBrowserProcessHandler
}

func init() {
	var _ capi.GetBrowserProcessHandlerHandler = (*myApp)(nil)
}

func (app *myApp) GetBrowserProcessHandler(self *capi.CAppT) *capi.CBrowserProcessHandlerT {
	return app.GetCBrowserProcessHandlerT()
}
