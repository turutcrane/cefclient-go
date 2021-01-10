package main

import (
	"log"

	"github.com/turutcrane/cefingo/capi"
	"github.com/turutcrane/cefingo/message_router"
)

type myRenderProcessHandler struct {
	capi.RefToCRenderProcessHandlerT
	router *router.RendererMessageRouter
}

func init() {
	var rph *myRenderProcessHandler

	var _ capi.OnContextCreatedHandler = rph
	var _ capi.CRenderProcessHandlerTOnProcessMessageReceivedHandler = rph
}

const (
	jsQueryFunctionName       = "cefQuery"
	jsQueryCancelFunctionName = "cefQueryCancel"
)

func (rph *myRenderProcessHandler) OnContextCreated(
	self *capi.CRenderProcessHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	context *capi.CV8contextT,
) {
	log.Println("T25:OnContextCreated")
	rph.router = router.RendererProcessOnContextCreated(routerMessagePrefix, context, jsQueryFunctionName, jsQueryCancelFunctionName)
}

func (rph *myRenderProcessHandler) OnProcessMessageReceived(
	self *capi.CRenderProcessHandlerT,
	browser *capi.CBrowserT,
	frame *capi.CFrameT,
	source_process capi.CProcessIdT,
	message *capi.CProcessMessageT,
) (ret bool) {
	if rph.router != nil {
		return rph.router.OnProcessMessageReceived(browser, frame, source_process, message)
	}
	return false
}
