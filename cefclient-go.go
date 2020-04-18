package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/cef"
)

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
	mainArgs := capi.NewCMainArgsT()
	mainArgs.SetWinHandle()

	life_span_handler := capi.AllocCLifeSpanHandlerT().Bind(&myLifeSpanHandler{})

	browser_process_handler := myBrowserProcessHandler{}
	capi.AllocCBrowserProcessHandlerT().Bind(&browser_process_handler)
	defer browser_process_handler.SetCBrowserProcessHandlerT(nil)

	client := capi.AllocCClientT().Bind(&myClient{})
	client.AssocLifeSpanHandlerT(life_span_handler)

	browser_process_handler.SetCClientT(client)

	app := capi.AllocCAppT().Bind(&myApp{})
	app.AssocBrowserProcessHandlerT(browser_process_handler.GetCBrowserProcessHandlerT())
	cef.ExecuteProcess(mainArgs, app)

	browser_process_handler.initial_url = flag.String("url", "https://www.golang.org/", "URL")
	flag.Parse() // should be after cef.ExecuteProcess() or implement CComandLine

	s := capi.NewCSettingsT()
	s.SetLogSeverity(capi.LogseverityWarning)
	s.SetNoSandbox(0)
	s.SetMultiThreadedMessageLoop(0)
	s.SetRemoteDebuggingPort(8088)

	cef.Initialize(mainArgs, s, app)
	runtime.UnlockOSThread()

	capi.RunMessageLoop()
	defer capi.Shutdown()
}

func init() {
	var _ capi.OnBeforeCloseHandler = myLifeSpanHandler{}
}

type myLifeSpanHandler struct {
}

func (myLifeSpanHandler) OnBeforeClose(self *capi.CLifeSpanHandlerT, brwoser *capi.CBrowserT) {
	capi.Logf("L89:")
	capi.QuitMessageLoop()
}

type myBrowserProcessHandler struct {
	// this reference forms an UNgabagecollectable circular reference
	// To GC, call myBrowserProcessHandler.SetCBrowserProcessHandlerT(nil)
	capi.RefToCBrowserProcessHandlerT

	capi.RefToCClientT
	initial_url *string
}

func (bph myBrowserProcessHandler) OnContextInitialized(sef *capi.CBrowserProcessHandlerT) {
	// factory := capi.AllocCSchemeHandlerFactoryT().Bind(&viewerSchemeHandlerFacgtory)
	// capi.RegisterSchemeHandlerFactory("http", internalHostname, factory)

	windowInfo := capi.NewCWindowInfoT()
	windowInfo.SetStyle(capi.WinWsOverlappedwindow | capi.WinWsClipchildren |
		capi.WinWsClipsiblings | capi.WinWsVisible)
	windowInfo.SetParentWindow(nil)
	windowInfo.SetX(capi.WinCwUseDefault)
	windowInfo.SetY(capi.WinCwUseDefault)
	windowInfo.SetWidth(capi.WinCwUseDefault)
	windowInfo.SetHeight(capi.WinCwUseDefault)
	windowInfo.SetWindowName("Vivliostyle Viewer")

	browserSettings := capi.NewCBrowserSettingsT()

	capi.BrowserHostCreateBrowser(windowInfo,
		bph.GetCClientT(),
		*bph.initial_url,
		browserSettings, nil, nil)
}

type myClient struct {
}

type myApp struct {
}
