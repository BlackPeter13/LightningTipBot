package main

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	walletConnectMessage = "🔗 *Link your wallet*\n\n" +
		"⚠️ Never share the URL or the QR code with anyone or they will be able to access your funds.\n\n" +
		"- *BlueWallet:* Press *New wallet*, *Import wallet*, *Scan or import a file*, and scan the QR code.\n" +
		"- *Zeus:* Copy the URL below, press *Add a new node*, *Import* (the URL), *Save Node Config*."
	couldNotLinkMessage = "🚫 Couldn't link your wallet. Please try again later."
)

func (bot TipBot) lndhubHandler(ctx context.Context, m *tb.Message) {
	if Configuration.Lnbits.LnbitsPublicUrl == "" {
		bot.trySendMessage(m.Sender, couldNotLinkMessage)
		return
	}
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	// first check whether the user is initialized
	fromUser := LoadUser(ctx)
	bot.trySendMessage(m.Sender, walletConnectMessage)

	lndhubUrl := fmt.Sprintf("lndhub://admin:%s@%slndhub/ext/", fromUser.Wallet.Adminkey, Configuration.Lnbits.LnbitsPublicUrl)

	// create qr code
	qr, err := qrcode.Encode(lndhubUrl, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err)
		log.Errorln(errmsg)
		return
	}

	// send the invoice data to user
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lndhubUrl)})
}
