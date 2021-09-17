package intercept

import (
	"context"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type CallbackFuncHandler func(ctx context.Context, message *tb.Callback)
type CallbackFunc func(ctx context.Context, message *tb.Callback) (context.Context, error)

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

func interceptCallback(ctx context.Context, message *tb.Callback, hm CallbackChain) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if hm != nil {
		var err error
		for _, m := range hm {
			ctx, err = m(ctx, message)
			if err != nil {
				return ctx, err
			}
		}
	}
	return ctx, nil
}

func HandlerWithCallback(handler CallbackFuncHandler, option ...CallbackInterceptOption) func(Callback *tb.Callback) {
	hm := &handlerCallbackInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(c *tb.Callback) {
		ctx, err := interceptCallback(context.Background(), c, hm.before)
		if err != nil {
			log.Errorln(err)
			return
		}
		hm.handler(ctx, c)
		_, err = interceptCallback(ctx, c, hm.after)
		if err != nil {
			log.Errorln(err)
		}
	}
}
