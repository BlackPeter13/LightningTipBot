package main

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"sync"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"

	"gorm.io/gorm"
)

type TipBot struct {
	database *gorm.DB
	bunt     *storage.DB
	logger   *gorm.DB
	telegram *telebot.Bot
	client   *lnbits.Client
}

var (
	paymentConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelPay            = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_pay")
	btnPay                  = paymentConfirmationMenu.Data("✅ Pay", "confirm_pay")
	sendConfirmationMenu    = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSend           = sendConfirmationMenu.Data("🚫 Cancel", "cancel_send")
	btnSend                 = sendConfirmationMenu.Data("✅ Send", "confirm_send")

	botWalletInitialisation     = sync.Once{}
	telegramHandlerRegistration = sync.Once{}
)

// NewBot migrates data and creates a new bot
func NewBot() TipBot {
	db, txLogger := migration()
	return TipBot{
		database: db,
		logger:   txLogger,
		bunt:     storage.NewBunt(Configuration.Database.BuntDbPath),
	}
}

// newTelegramBot will create a new telegram bot.
func newTelegramBot() *tb.Bot {
	tgb, err := tb.NewBot(tb.Settings{
		Token:     Configuration.Telegram.ApiKey,
		Poller:    &tb.LongPoller{Timeout: 60 * time.Second},
		ParseMode: tb.ModeMarkdown,
	})
	if err != nil {
		panic(err)
	}
	return tgb
}

// initBotWallet will create / initialize the bot wallet
// todo -- may want to derive user wallets from this specific bot wallet (master wallet), since lnbits usermanager extension is able to do that.
func (bot TipBot) initBotWallet() error {
	botWalletInitialisation.Do(func() {
		_, err := bot.initWallet(bot.telegram.Me)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initBotWallet] Could not initialize bot wallet: %s", err.Error()))
			return
		}
	})
	return nil
}

type CommandEndpoint struct {
}
type Handler struct {
	Endpoint    []interface{}
	Handler     interface{}
	Interceptor *Interceptor
}

// registerTelegramHandlers will register all telegram handlers.
func (bot TipBot) registerTelegramHandlers() {
	telegramHandlerRegistration.Do(func() {
		// Set up handlers
		var handler = []Handler{
			{
				Endpoint: []interface{}{"/start"},
				Handler:  bot.startHandler,
			},
			{
				Endpoint:    []interface{}{"/faucet", "/zapfhahn", "/kraan"},
				Handler:     bot.faucetHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/tip"},
				Handler:     bot.tipHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor, bot.loadReplyToInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/pay"},
				Handler:     bot.confirmPaymentHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/invoice"},
				Handler:     bot.invoiceHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/balance"},
				Handler:     bot.balanceHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/send"},
				Handler:     bot.confirmSendHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/help"},
				Handler:     bot.helpHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/basics"},
				Handler:     bot.basicsHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/donate"},
				Handler:     bot.donationHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/advanced"},
				Handler:     bot.advancedHelpHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/link"},
				Handler:     bot.lndhubHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{"/lnurl"},
				Handler:     bot.lnurlHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{tb.OnPhoto},
				Handler:     bot.privatePhotoHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{tb.OnText},
				Handler:     bot.anyTextHandler,
				Interceptor: &Interceptor{Type: MessageInterceptor, BeforeMessage: []intercept.MessageFunc{bot.loadUserInterceptor}},
			},
			{
				Endpoint:    []interface{}{tb.OnQuery},
				Handler:     bot.anyQueryHandler,
				Interceptor: &Interceptor{Type: QueryInterceptor, BeforeQuery: []intercept.QueryFunc{bot.loadUserQueryInterceptor}},
			},
			{
				Endpoint: []interface{}{tb.OnChosenInlineResult},
				Handler:  bot.anyChosenInlineHandler,
			},
			{
				Endpoint:    []interface{}{&btnPay},
				Handler:     bot.payHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint:    []interface{}{&btnCancelPay},
				Handler:     bot.cancelPaymentHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint:    []interface{}{&btnSend},
				Handler:     bot.sendHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint:    []interface{}{&btnCancelSend},
				Handler:     bot.cancelSendHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint:    []interface{}{&btnAcceptInlineSend},
				Handler:     bot.acceptInlineSendHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint: []interface{}{&btnCancelInlineSend},
				Handler:  bot.cancelInlineSendHandler,
			},
			{
				Endpoint:    []interface{}{&btnAcceptInlineReceive},
				Handler:     bot.acceptInlineReceiveHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint: []interface{}{&btnCancelInlineReceive},
				Handler:  bot.cancelInlineReceiveHandler,
			},
			{
				Endpoint:    []interface{}{&btnAcceptInlineFaucet},
				Handler:     bot.accpetInlineFaucetHandler,
				Interceptor: &Interceptor{Type: CallbackInterceptor, BeforeCallback: []intercept.CallbackFunc{bot.loadUserCallbackInterceptor}},
			},
			{
				Endpoint: []interface{}{&btnCancelInlineFaucet},
				Handler:  bot.cancelInlineFaucetHandler,
			},
		}

		for _, h := range handler {
			for _, endpoint := range h.Endpoint {
				fmt.Println("registering", endpoint)
				if h.Interceptor != nil {
					switch h.Interceptor.Type {
					case MessageInterceptor:
						bot.telegram.Handle(endpoint,
							intercept.HandlerWithMessage(h.Handler.(func(ctx context.Context, query *tb.Message)),
								intercept.WithBeforeMessage(h.Interceptor.BeforeMessage...),
								intercept.WithAfterMessage(h.Interceptor.AfterMessage...)))

					case QueryInterceptor:
						bot.telegram.Handle(endpoint,
							intercept.HandlerWithQuery(h.Handler.(func(ctx context.Context, query *tb.Query)),
								intercept.WithBeforeQuery(h.Interceptor.BeforeQuery...),
								intercept.WithAfterQuery(h.Interceptor.AfterQuery...)))

					case CallbackInterceptor:
						bot.telegram.Handle(endpoint,
							intercept.HandlerWithCallback(h.Handler.(func(ctx context.Context, callback *tb.Callback)),
								intercept.WithBeforeCallback(h.Interceptor.BeforeCallback...),
								intercept.WithAfterCallback(h.Interceptor.AfterCallback...)))

					}
				} else {
					bot.telegram.Handle(endpoint, h.Handler)
				}

			}
		}

	})
}

// Start will initialize the telegram bot and lnbits.
func (bot TipBot) Start() {
	// set up lnbits api
	bot.client = lnbits.NewClient(Configuration.Lnbits.AdminKey, Configuration.Lnbits.Url)
	// set up telebot
	bot.telegram = newTelegramBot()
	log.Infof("[Telegram] Authorized on account @%s", bot.telegram.Me.Username)
	// initialize the bot wallet
	err := bot.initBotWallet()
	if err != nil {
		log.Errorf("Could not initialize bot wallet: %s", err.Error())
	}
	bot.registerTelegramHandlers()
	lnbits.NewWebhookServer(Configuration.Lnbits.WebhookServerUrl, bot.telegram, bot.client, bot.database)
	lnurl.NewServer(Configuration.Bot.LNURLServerUrl, Configuration.Bot.LNURLHostUrl, Configuration.Lnbits.WebhookServer, bot.telegram, bot.client, bot.database)
	bot.telegram.Start()
}
