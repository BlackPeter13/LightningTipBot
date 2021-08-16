package main

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

type Message struct {
	Message   *tb.Message
	TipAmount int
	Ntips     int
	LastTip   time.Time
	Tippers   []*tb.User
}

const maxNamesInTipperMessage = 5

type messageOption func(m *Message)

func TipAmount(amount int) messageOption {
	return func(m *Message) {
		m.TipAmount = amount
	}
}
func Tips(nTips int) messageOption {
	return func(m *Message) {
		m.LastTip = time.Now()
		m.Ntips = nTips
	}
}

func NewMessage(m *tb.Message, opts ...messageOption) *Message {
	msg := &Message{
		Message: m,
		Tippers: make([]*tb.User, 0),
	}
	for _, opt := range opts {
		opt(msg)
	}
	return msg

}

// updateToolTip updates existing tip tool tip in telegram
func (x *Message) updateTooltip(bot *TipBot, user *tb.User, amount int, notInitializedWallet bool) {
	x.TipAmount += amount
	x.Ntips += 1
	x.Tippers = appendUinqueUsersToSlice(x.Tippers, user)
	x.LastTip = time.Now()
	userTips := bot.tips[x.Message.ReplyTo.Sender.ID]
	for _, tip := range userTips {
		if tip.Message.ReplyTo.Sender.ID == x.Message.ReplyTo.Sender.ID {
			err := x.editTooltip(bot, notInitializedWallet)
			if err != nil {
				log.Printf("[updateTooltip] could not edit tooltip: %s", err.Error())
				continue
			}
			tip = x
		}
	}
	bot.tips[x.Message.ReplyTo.Sender.ID] = userTips
}
func (x *Message) editTooltip(bot *TipBot, notInitializedWallet bool) error {
	tipToolTip := x.getTooltipMessage(GetUserStrMd(bot.telegram.Me), notInitializedWallet)
	m, err := bot.telegram.Edit(x.Message, tipToolTip)
	if err != nil {
		return err
	}
	x.Message.Text = m.Text
	return nil
}
func tipTooltipInitializedHandler(user *tb.User, bot TipBot) {
	for _, tip := range bot.tips[user.ID] {
		if tip.Message.ReplyTo.Sender.ID == user.ID {
			err := tip.editTooltip(&bot, false)
			if err != nil {
				log.Printf("[tipTooltipInitializedHandler] could not edit tooltip: %s", err.Error())
				continue
			}
		}
	}
}

// getTippersString joins all tippers username or telegram id's as mentions (@username or [inline mention of a user](tg://user?id=123456789))
func getTippersString(tippers []*tb.User) string {
	var tippersStr string
	for _, uniqueUser := range tippers {
		userStr := GetUserStrMd(uniqueUser)
		tippersStr += fmt.Sprintf("%s, ", userStr)
	}
	// get rid of the trailing comma
	if len(tippersStr) > 2 {
		tippersStr = tippersStr[:len(tippersStr)-2]
	}
	tippersSlice := strings.Split(tippersStr, " ")
	// crop the message to the max length
	if len(tippersSlice) > maxNamesInTipperMessage {
		// tippersStr = tippersStr[:50]
		tippersStr = strings.Join(tippersSlice[:maxNamesInTipperMessage], " ")
		tippersStr = tippersStr + " ... and others"
	}
	return tippersStr
}

// getTooltipMessage will return the full tip tool tip
func (x Message) getTooltipMessage(botUserName string, notInitializedWallet bool) string {
	tippersStr := getTippersString(x.Tippers)
	tipToolTipMessage := fmt.Sprintf("🏅 %d sat", x.TipAmount)
	if len(x.Tippers) > 1 {
		tipToolTipMessage = fmt.Sprintf("%s (%d tips by %s)", tipToolTipMessage, x.Ntips, tippersStr)
	} else {
		tipToolTipMessage = fmt.Sprintf("%s (by %s)", tipToolTipMessage, tippersStr)
	}

	if notInitializedWallet {
		tipToolTipMessage = tipToolTipMessage + fmt.Sprintf("\n🗑 Chat with %s to manage your wallet.", botUserName)
	}
	return tipToolTipMessage
}

// tipTooltipExists checks if this tip is already known
func tipTooltipExists(m *tb.Message, bot *TipBot) (bool, *Message) {
	for _, tip := range bot.tips[m.ReplyTo.Sender.ID] {
		if tip.Message.ReplyTo != nil && m.ReplyTo != nil {
			if tip.Message.ReplyTo.ID == m.ReplyTo.ID {
				return true, tip
			}
		}
	}
	return false, nil
}

// tipTooltipHandler function to update the tooltip below a tipped message. either updates or creates initial tip tool tip
func tipTooltipHandler(m *tb.Message, user *tb.User, bot *TipBot, amount int, notInitializedWallet bool) bool {
	// todo: this crashes if the tooltip message (maybe also the original tipped message) was deleted in the mean time!!! need to check for existence!
	ok, tipMessage := tipTooltipExists(m, bot)
	if ok {
		// update the tooltip with new tippers
		tipMessage.updateTooltip(bot, user, amount, notInitializedWallet)
	} else {
		tipmsg := fmt.Sprintf("🏅 %d sat", amount)
		userStr := GetUserStrMd(user)
		tipmsg = fmt.Sprintf("%s (by %s)", tipmsg, userStr)

		if notInitializedWallet {
			tipmsg = tipmsg + fmt.Sprintf("\n🗑 Chat with %s to manage your wallet.", GetUserStrMd(bot.telegram.Me))
		}
		msg, err := bot.telegram.Reply(m.ReplyTo, tipmsg)
		if err != nil {
			print(err)
		}
		message := NewMessage(msg, TipAmount(amount), Tips(1))
		message.Tippers = appendUinqueUsersToSlice(message.Tippers, user)
		bot.tips[m.ReplyTo.Sender.ID] = append(bot.tips[m.ReplyTo.Sender.ID], message)
	}
	// first call will return false, every following call will return true
	return ok
}

func (x Message) Dispose(duration time.Duration, telegram *tb.Bot) {
	// do not delete messages from private chat
	if x.Message.Private() {
		return
	}
	go func() {
		time.Sleep(duration)
		err := telegram.Delete(x.Message)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}()
}
