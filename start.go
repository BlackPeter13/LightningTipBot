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

const (
	startSettingWalletMessage = "🧮 Setting up your wallet..."
	startWalletReadyMessage   = "✅ *Your wallet is ready.*"
)

func (bot TipBot) startHandler(m *tb.Message) {
	if !m.Private() {
		return
	}
	bot.helpHandler(m)
	log.Printf("[/start] User: %s (%d)\n", m.Sender.Username, m.Sender.ID)

	walletCreationMsg, err := bot.telegram.Send(m.Sender, startSettingWalletMessage)
	err = bot.initWallet(m.Sender)
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
		return
	}
	bot.telegram.Edit(walletCreationMsg, startWalletReadyMessage)
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
