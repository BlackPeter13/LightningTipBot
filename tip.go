package main

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	tipDidYouReplyMessage = "Did you reply to a message to tip? To reply to any message, right-click -> Reply on your computer or swipe the message on your phone. If you want to send directly to another user, use the /send command."
	tipInviteGroupMessage = "ℹ️ By the way, you can invite this bot to any group to start tipping there."
	tipEnterAmountMessage = "Did you enter an amount?"
	tipValidAmountMessage = "Did you enter a valid amount?"
	tipYourselfMessage    = "📖 You can't tip yourself."
	tipSentMessage        = "💸 %d sat sent to %s."
	tipReceivedMessage    = "🏅 %s has tipped you %d sat."
	tipErrorMessage       = "🚫 Transaction failed: %s"
	tipUndefinedErrorMsg  = "please try again later"
	tipHelpText           = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/tip <amount> [<memo>]`\n" +
		"*Example:* `/tip 1000 Dank meme!`"
)

func helpTipUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(tipHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(tipHelpText, "")
	}
}

func TipCheckSyntax(m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, tipEnterAmountMessage
	}
	return true, ""
}

func (bot *TipBot) tipHandler(ctx context.Context, m *tb.Message) {
	// delete the tip message after a few seconds, this is default behaviour
	defer NewMessage(m, WithDuration(time.Second*time.Duration(Configuration.Telegram.MessageDisposeDuration), bot.telegram))
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// only if message is a reply
	if !m.IsReply() {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpTipUsage(fmt.Sprintf(tipDidYouReplyMessage)))
		bot.trySendMessage(m.Sender, tipInviteGroupMessage)
		return
	}

	if ok, err := TipCheckSyntax(m); !ok {
		bot.trySendMessage(m.Sender, helpTipUsage(err))
		NewMessage(m, WithDuration(0, bot.telegram))
		return
	}

	// get tip amount
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil || amount < 1 {
		errmsg := fmt.Sprintf("[/tip] Error: Tip amount not valid.")
		// immediately delete if the amount is bullshit
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpTipUsage(tipValidAmountMessage))
		log.Errorln(errmsg)
		return
	}

	err = bot.parseCmdDonHandler(ctx, m)
	if err == nil {
		return
	}
	// TIP COMMAND IS VALID
	from := ctx.Value("user").(*lnbits.User)

	to := ctx.Value("reply_to_user").(*lnbits.User)

	if from.ID == to.ID {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, tipYourselfMessage)
		return
	}

	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(from.Telegram)

	if _, exists := bot.UserExists(to.Telegram); !exists {
		log.Infof("[/tip] User %s has no wallet.", toUserStr)
		to, err = bot.CreateWalletForTelegramUser(to.Telegram)
		if err != nil {
			errmsg := fmt.Errorf("[/tip] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}

	// check for memo in command
	tipMemo := ""
	if len(strings.Split(m.Text, " ")) > 2 {
		tipMemo = strings.SplitN(m.Text, " ", 3)[2]
		if len(tipMemo) > 200 {
			tipMemo = tipMemo[:200]
			tipMemo = tipMemo + "..."
		}
	}

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Tip from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("tip"), TransactionChat(m.Chat))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		NewMessage(m, WithDuration(0, bot.telegram))
		if err != nil {
			bot.trySendMessage(m.Sender, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(m.Sender, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[/tip] Transaction failed: %s", err)
		log.Errorln(errMsg)
		return
	}

	// update tooltip if necessary
	messageHasTip := tipTooltipHandler(m, bot, amount, to.Initialized)

	log.Infof("[tip] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	// notify users
	_, err = bot.telegram.Send(from.Telegram, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[/tip] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// forward tipped message to user once
	if !messageHasTip {
		bot.tryForwardMessage(to.Telegram, m.ReplyTo, tb.Silent)
	}
	bot.trySendMessage(to.Telegram, fmt.Sprintf(tipReceivedMessage, fromUserStrMd, amount))

	if len(tipMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", MarkdownEscape(tipMemo)))
	}
	return
}
