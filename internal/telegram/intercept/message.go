package intercept

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

type MessageInterface interface {
	a() func(ctx context.Context, message *tb.Message)
}
type MessageFuncHandler func(ctx context.Context, message *tb.Message)
type MessageFunc func(ctx context.Context, message *tb.Message) context.Context

type handlerMessageInterceptor struct {
	handler MessageFuncHandler
	before  MessageChain
	after   MessageChain
}
type MessageChain []MessageFunc
type MessageInterceptOption func(*handlerMessageInterceptor)

func WithBeforeMessage(chain ...MessageFunc) MessageInterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.before = chain
	}
}
func WithAfterMessage(chain ...MessageFunc) MessageInterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.after = chain
	}
}

func interceptMessage(ctx context.Context, message *tb.Message, hm MessageChain) context.Context {
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

/*func Handle(handler interface{}, option ...MessageInterceptOption) func(message *tb.Message) {
	switch handler.(type) {
	case func(ctx context.Context, message *tb.Message):
		handler.(func(ctx context.Context, message *tb.Message))()
	}
}*/
func HandlerWithMessage(handler MessageFuncHandler, option ...MessageInterceptOption) func(message *tb.Message) {

	hm := &handlerMessageInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(message *tb.Message) {
		ctx := interceptMessage(context.Background(), message, hm.before)
		hm.handler(ctx, message)
		interceptMessage(ctx, message, hm.after)
	}
}
