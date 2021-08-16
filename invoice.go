package main

import (
	"bytes"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/tucnak/telebot.v2"
)

func helpInvoiceUsage(errormsg string) string {
	helpstr := "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/invoice <amount> [<memo>]`\n" +
		"*Example:* `/invoice 1000 Take this! 💸`"
	if len(errormsg) > 0 {
		helpstr = fmt.Sprintf(helpstr, fmt.Sprintf("_%s_", errormsg))
	} else {
		helpstr = fmt.Sprintf(helpstr, "")
	}
	return helpstr
}

func (bot TipBot) invoiceHandler(m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		bot.telegram.Send(m.Sender, helpInvoiceUsage(""))
		return
	}

	user, err := GetUser(m.Sender, bot)
	userStr := GetUserStr(m.Sender)
	amount, err := DecodeAmountFromCommand(m.Text)
	if err != nil {
		return
	}
	if amount > 0 {
	} else {
		bot.telegram.Send(m.Sender, helpInvoiceUsage("Did you use a valid amount?"))
		return
	}

	// check for memo in command
	memo := "Powered by @LightningTipBot"
	if len(strings.Split(m.Text, " ")) > 2 {
		memo = strings.SplitN(m.Text, " ", 3)[2]
		tag := " (@LightningTipBot)"
		memoMaxLen := 159 - len(tag)
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen-len(tag)]
		}
		memo = memo + tag
	}

	log.Infof("[/invoice] Creating invoice for %s of %d sat.", userStr, amount)
	// generate invoice
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  int64(amount),
			Memo:    memo,
			Webhook: Configuration.WebhookServer},
		*user.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err)
		log.Errorln(errmsg)
		return
	}

	// create qr code
	qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err)
		log.Errorln(errmsg)
		return
	}

	// send the invoice data to user
	bot.telegram.Send(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
	log.Printf("[/invoice] Incvoice created. User: %s, amount: %d sat. Payment request: %s", userStr, amount, invoice.PaymentHash)
	return
}

// func helpPayInvoiceUsage(errormsg string) string {
// 	helpstr := "📖 Oops, that didn't work. %s\n\n" +
// 		"*Usage:* `/pay <invoice>`\n" +
// 		"*Example:* `/pay lnbc20n1psscehdp68cs08a5cjmy8q...`"
// 	if len(errormsg) > 0 {
// 		helpstr = fmt.Sprintf(helpstr, fmt.Sprintf("_%s_", errormsg))
// 	} else {
// 		helpstr = fmt.Sprintf(helpstr, "")
// 	}
// 	return helpstr
// }

// func (bot TipBot) payInvoiceHandler(m *tb.Message) {
// 	if m.Chat.Type != tb.ChatPrivate {
// 		// delete message
// 		DisposeMessage(m, time.Second*0).Dispose(bot.telegram)
// 		bot.telegram.Send(m.Sender, helpPayInvoiceUsage("You can pay invoices only in the private chat with the bot."))
// 		return
// 	}
// 	if len(strings.Split(m.Text, " ")) < 2 {
// 		DisposeMessage(m, time.Second*0).Dispose(bot.telegram)
// 		bot.telegram.Send(m.Sender, helpPayInvoiceUsage(""))
// 		return
// 	}
// 	user, err := GetUser(m.Sender, bot)
// 	payment_request, err := getArgumentFromCommand(m.Text, 1)

// 	// decode invoice
// 	bolt11, err := decodepay.Decodepay(payment_request)
// 	if err != nil {
// 		errmsg := fmt.Sprintf("[/pay] Error: Could not decode invoice")
// 		bot.telegram.Send(m.Sender, helpPayInvoiceUsage("Did you enter a valid Lightning invoice?"))
// 		log.Errorln(errmsg)
// 		return
// 	}
// 	amount := int(bolt11.MSatoshi / 1000)
// 	// description := bolt11.Description
// 	// log.Printf("[Pay Invoice] Description: %s", description)

// 	// check user balance first
// 	balance, err := bot.GetUserBalance(m.Sender)
// 	if err != nil {
// 		DisposeMessage(m, time.Second*0).Dispose(bot.telegram)
// 		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance")
// 		log.Errorln(errmsg)
// 		return
// 	}
// 	if amount > balance {
// 		DisposeMessage(m, time.Second*0).Dispose(bot.telegram)
// 		bot.telegram.Send(m.Sender, helpPayInvoiceUsage("Insufficient funds. You have %d sat but need %d sat."))
// 		return
// 	}

// 	log.Printf("[/pay] User: %s, amount: %d sat.", m.Sender.Username, amount)
// 	// pay invoice
// 	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: payment_request}, *user.Wallet)
// 	if err != nil {
// 		errmsg := fmt.Sprintf("[/pay] ould not pay invoice of user %s: %s", m.Sender.Username, err)
// 		bot.telegram.Send(m.Sender, fmt.Sprintf("❌ Failed to pay invoice: %s", err))
// 		log.Errorln(errmsg)
// 		return
// 	}
// 	bot.telegram.Send(m.Sender, "⚡️ Invoice paid.")
// 	return
// }
