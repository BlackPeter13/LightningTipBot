package main

import (
	"errors"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/gorm"
)

func helpHowtoUse() string {
	return "ℹ️ *Info*\n_This bot sends Bitcoin tips on the Lightning Network⚡️. The basic unit of tips are Satoshis (sat). 100,000,000 sat = 1 Bitcoin. There will only ever be 21 Million Bitcoin._\n\n" +
		"❤️ *Donate*\n" +
		"_This bot charges no fees. If you like to support this bot, please consider a donation to cover operational costs. To donate, just tip @LightningTipBot or try_ `/send 1000 @LightningTipBot`\n\n" +
		"📖 *Commands*\n" +
		"*/tip* 🏅 Reply to a message to tip it: `/tip <amount> [<memo>]`\n" +
		"*/balance* 👑 Check your balance: `/balance`\n" +
		"*/send* 💸 Send funds to a user: `/send <amount> <@username> [<memo>]`\n" +
		"*/invoice* ⚡️ Receive over Lightning: `/invoice <amount> [<memo>]`\n" +
		"*/pay* ⚡️ Pay over Lightning: `/pay <invoice>`\n" +
		"*/help* 📖 Read this help.\n"
}

func (bot TipBot) helpHandler(m *tb.Message) {
	if !m.Private() {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
	}
	bot.telegram.Send(m.Sender, helpHowtoUse(), tb.NoPreview)
	return
}

func (bot TipBot) startHandler(m *tb.Message) {
	if !m.Private() {
		return
	}
	bot.telegram.Send(m.Sender, helpHowtoUse(), tb.NoPreview)
	log.Printf("[/start] %s (%d)\n", m.Sender.Username, m.Sender.ID)

	walletCreationMsg, err := bot.telegram.Send(m.Sender, "🧮 Setting up your wallet...")
	err = bot.initWallet(m.Sender)
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
		return
	}
	bot.telegram.Edit(walletCreationMsg, "✅ *Your wallet is ready.*")
	bot.balanceHandler(m)
	return
}

func (bot TipBot) initWallet(tguser *tb.User) error {
	user, err := GetUser(tguser, bot)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u := &lnbits.User{Telegram: tguser}
		err = bot.createWallet(u)
		if err != nil {
			return err
		}
		u.Initialized = true
		err = UpdateUserRecord(u, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initWallet] error updating user: %s", err.Error()))
			return err
		}
	} else if !user.Initialized {
		tipTooltipInitializedHandler(user.Telegram, bot)
		user.Initialized = true
		err = UpdateUserRecord(user, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initWallet] error updating user: %s", err.Error()))
			return err
		}
	}
	return nil
}

func (bot TipBot) createWallet(user *lnbits.User) error {
	UserStr := GetUserStr(user.Telegram)
	u, err := bot.client.CreateUserWithInitialWallet(strconv.Itoa(user.Telegram.ID),
		fmt.Sprintf("%d (%s)", user.Telegram.ID, UserStr),
		Configuration.AdminKey,
		UserStr)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Create wallet error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &lnbits.Wallet{Client: bot.client}
	user.ID = u.ID
	user.Name = u.Name
	wallet, err := user.Wallet.Wallets(*user)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Get wallet error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &wallet[0]
	user.Wallet.Client = bot.client
	user.Initialized = false
	err = UpdateUserRecord(user, bot)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Update user record error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	return nil
}
