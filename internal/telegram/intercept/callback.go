package intercept

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

type CallbackFuncHandler func(ctx context.Context, message *tb.Callback)
type CallbackFunc func(ctx context.Context, message *tb.Callback) context.Context

type handlerCallbackInterceptor struct {
	handler CallbackFuncHandler
	before  CallbackChain
	after   CallbackChain
}
type CallbackChain []CallbackFunc
type CallbackInterceptOption func(*handlerCallbackInterceptor)

func WithBeforeCallback(chain ...CallbackFunc) CallbackInterceptOption {
	return func(a *handlerCallbackInterceptor) {
		a.before = chain
	}
}
func WithAfterCallback(chain ...CallbackFunc) CallbackInterceptOption {
	return func(a *handlerCallbackInterceptor) {
		a.after = chain
	}
}

func interceptCallback(ctx context.Context, message *tb.Callback, hm CallbackChain) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if hm != nil {
		for _, m := range hm {
			ctx = m(ctx, message)
		}
	}
	return ctx
}

func HandlerWithCallback(handler CallbackFuncHandler, option ...CallbackInterceptOption) func(Callback *tb.Callback) {
	hm := &handlerCallbackInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(c *tb.Callback) {
		ctx := interceptCallback(context.Background(), c, hm.before)
		hm.handler(ctx, c)
		interceptCallback(ctx, c, hm.after)
	}
}
