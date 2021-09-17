package main

import (
	"context"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	tb "gopkg.in/tucnak/telebot.v2"
)

type InterceptorType int

const (
	MessageInterceptor InterceptorType = iota
	CallbackInterceptor
	QueryInterceptor
)

type Interceptor struct {
	Type           InterceptorType
	BeforeMessage  []intercept.MessageFunc
	AfterMessage   []intercept.MessageFunc
	BeforeQuery    []intercept.QueryFunc
	AfterQuery     []intercept.QueryFunc
	BeforeCallback []intercept.CallbackFunc
	AfterCallback  []intercept.CallbackFunc
}

// loadUserQueryInterceptor Loading the telegram user with query intercept
func (bot TipBot) loadUserQueryInterceptor(ctx context.Context, c *tb.Query) context.Context {
	user, err := GetUser(&c.From, bot)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, "user", user)
}

// loadUserCallbackInterceptor Loading the telegram user with callback intercept
func (bot TipBot) loadUserCallbackInterceptor(ctx context.Context, c *tb.Callback) context.Context {
	m := *c.Message
	m.Sender = c.Sender
	return bot.loadUserInterceptor(ctx, &m)
}

// loadReplyToInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(ctx context.Context, m *tb.Message) context.Context {
	if m.ReplyTo != nil {
		if m.ReplyTo.Sender != nil {
			user, _ := GetUser(m.ReplyTo.Sender, bot)
			user.Telegram = m.ReplyTo.Sender
			return context.WithValue(ctx, "reply_to_user", user)
		}
	}
	return context.Background()
}

// loadUserInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadUserInterceptor(ctx context.Context, m *tb.Message) context.Context {
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, "user", user)
}

func LoadUser(ctx context.Context) *lnbits.User {
	u := ctx.Value("user")
	if u != nil {
		return u.(*lnbits.User)
	}
	return nil
}
