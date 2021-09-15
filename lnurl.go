package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	lnurlReceiveInfoText           = "👇 You can use this LNURL to receive payments."
	lnurlResolvingUrlMessage       = "🧮 Resolving address..."
	lnurlGettingUserMessage        = "🧮 Preparing payment..."
	lnurlPaymentFailed             = "🚫 Payment failed: %s"
	lnurlInvalidAmountMessage      = "🚫 Invalid amount."
	lnurlInvalidAmountRangeMessage = "🚫 Amount must be between %d and %d sat."
	lnurlNoUsernameMessage         = "🚫 You need to set a Telegram username to receive via LNURL."
	lnurlEnterAmountMessage        = "⌨️ Enter an amount between %d and %d sat."
	lnurlHelpText                  = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/lnurl [amount] <lnurl>`\n" +
		"*Example:* `/lnurl LNURL1DP68GUR...`"
)

// lnurlHandler is invoked on /lnurl command
func (bot TipBot) lnurlHandler(ctx context.Context, m *tb.Message) {
	// commands:
	// /lnurl
	// /lnurl <LNURL>
	// or /lnurl <amount> <LNURL>
	log.Infof("[lnurlHandler] %s", m.Text)

	// if only /lnurl is entered, show the lnurl of the user
	if m.Text == "/lnurl" {
		bot.lnurlReceiveHandler(m)
		return
	}

	// assume payment
	// HandleLNURL by fiatjaf/go-lnurl
	msg := bot.trySendMessage(m.Sender, lnurlResolvingUrlMessage)
	_, params, err := HandleLNURL(m.Text)
	if err != nil {
		bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, "could not resolve LNURL."))
		log.Errorln(err)
		return
	}
	var payParams LnurlStateResponse
	switch params.(type) {
	case lnurl.LNURLPayResponse1:
		payParams = LnurlStateResponse{LNURLPayResponse1: params.(lnurl.LNURLPayResponse1)}
		log.Infof("[lnurlHandler] %s", payParams.Callback)
	default:
		err := fmt.Errorf("invalid LNURL type.")
		log.Errorln(err)
		bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	user := LoadUser(ctx)

	// if no amount is in the command, ask for it
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil || amount < 1 {
		// set LNURLPayResponse1 in the state of the user
		paramsJson, err := json.Marshal(payParams)
		if err != nil {
			log.Errorln(err)
			return
		}

		SetUserState(user, bot, lnbits.UserStateLNURLEnterAmount, string(paramsJson))

		bot.tryDeleteMessage(msg)
		// Let the user enter an amount and return
		bot.trySendMessage(m.Sender, fmt.Sprintf(lnurlEnterAmountMessage, payParams.MinSendable/1000, payParams.MaxSendable/1000), tb.ForceReply)
	} else {
		// amount is already present in the command
		// set also amount in the state of the user
		// todo: this is repeated code, could be shorter
		payParams.Amount = amount
		paramsJson, err := json.Marshal(payParams)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(m.Sender, err.Error())
			return
		}
		SetUserState(user, bot, lnbits.UserStateConfirmLNURLPay, string(paramsJson))
		bot.tryDeleteMessage(msg)
		// directly go to confirm
		bot.lnurlPayHandler(ctx, m)
	}
}

func (bot *TipBot) UserGetLightningAddress(user *tb.User) (string, error) {
	if len(user.Username) > 0 {
		return fmt.Sprintf("%s@%s", strings.ToLower(user.Username), strings.ToLower(Configuration.Bot.LNURLHostUrl.Hostname())), nil
	} else {
		return "", fmt.Errorf("user has no username")
	}
}

func UserGetLNURL(user *tb.User) (string, error) {
	name := strings.ToLower(strings.ToLower(user.Username))
	if len(name) == 0 {
		return "", fmt.Errorf("user has no username.")
	}
	callback := fmt.Sprintf("%s/.well-known/lnurlp/%s", Configuration.Bot.LNURLHostName, name)
	log.Debugf("[lnurlReceiveHandler] %s's LNURL: %s", GetUserStr(user), callback)

	lnurlEncode, err := lnurl.LNURLEncode(callback)
	if err != nil {
		return "", err
	}
	return lnurlEncode, nil
}

// lnurlReceiveHandler outputs the LNURL of the user
func (bot TipBot) lnurlReceiveHandler(m *tb.Message) {
	lnurlEncode, err := UserGetLNURL(m.Sender)
	if err != nil {
		errmsg := fmt.Sprintf("[lnurlReceiveHandler] Failed to get LNURL: %s", err)
		log.Errorln(errmsg)
		bot.telegram.Send(m.Sender, lnurlNoUsernameMessage)
	}
	// create qr code
	qr, err := qrcode.Encode(lnurlEncode, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[lnurlReceiveHandler] Failed to create QR code for LNURL: %s", err)
		log.Errorln(errmsg)
		return
	}

	bot.trySendMessage(m.Sender, lnurlReceiveInfoText)
	// send the lnurl data to user
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lnurlEncode)})
}

func (bot TipBot) lnurlEnterAmountHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.StateKey == lnbits.UserStateLNURLEnterAmount {
		a, err := strconv.Atoi(m.Text)
		if err != nil {
			log.Errorln(err)
			bot.trySendMessage(m.Sender, lnurlInvalidAmountMessage)
			ResetUserState(user, bot)
			return
		}
		amount := int64(a)
		var stateResponse LnurlStateResponse
		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
		if err != nil {
			log.Errorln(err)
			ResetUserState(user, bot)
			return
		}
		// amount not in allowed range from LNURL
		if amount > (stateResponse.MaxSendable/1000) || amount < (stateResponse.MinSendable/1000) {
			err = fmt.Errorf("amount not in range")
			log.Errorln(err)
			bot.trySendMessage(m.Sender, fmt.Sprintf(lnurlInvalidAmountRangeMessage, stateResponse.MinSendable/1000, stateResponse.MaxSendable/1000))
			ResetUserState(user, bot)
			return
		}
		stateResponse.Amount = a
		state, err := json.Marshal(stateResponse)
		if err != nil {
			log.Errorln(err)
			ResetUserState(user, bot)
			return
		}
		SetUserState(user, bot, lnbits.UserStateConfirmLNURLPay, string(state))
		bot.lnurlPayHandler(ctx, m)
	}
}

// LnurlStateResponse saves the state of the user for an LNURL payment
type LnurlStateResponse struct {
	lnurl.LNURLPayResponse1
	Amount int `json:"amount"`
}

// lnurlPayHandler is invoked when the user has delivered an amount and is ready to pay
func (bot TipBot) lnurlPayHandler(ctx context.Context, c *tb.Message) {
	msg := bot.trySendMessage(c.Sender, lnurlGettingUserMessage)

	user := LoadUser(ctx)
	if user.StateKey == lnbits.UserStateConfirmLNURLPay {
		client, err := getHttpClient()
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
			return
		}
		var stateResponse LnurlStateResponse
		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
			return
		}
		callbackUrl, err := url.Parse(stateResponse.Callback)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
			return
		}
		qs := callbackUrl.Query()
		qs.Set("amount", strconv.Itoa(stateResponse.Amount*1000))
		callbackUrl.RawQuery = qs.Encode()

		res, err := client.Get(callbackUrl.String())
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
			return
		}
		var response2 lnurl.LNURLPayResponse2
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, err))
			return
		}
		json.Unmarshal(body, &response2)

		if len(response2.PR) < 1 {
			bot.tryEditMessage(msg, fmt.Sprintf(lnurlPaymentFailed, "could not receive invoice (wrong address?)."))
			return
		}
		bot.telegram.Delete(msg)
		c.Text = fmt.Sprintf("/pay %s", response2.PR)
		bot.confirmPaymentHandler(ctx, c)
	}
}

func getHttpClient() (*http.Client, error) {
	client := http.Client{}
	if Configuration.Bot.HttpProxy != "" {
		proxyUrl, err := url.Parse(Configuration.Bot.HttpProxy)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}
	return &client, nil
}
func (bot TipBot) cancelLnUrlHandler(c *tb.Callback) {

}

// from https://github.com/fiatjaf/go-lnurl
func HandleLNURL(rawlnurl string) (string, lnurl.LNURLParams, error) {
	var err error
	var rawurl string

	if name, domain, ok := lnurl.ParseInternetIdentifier(rawlnurl); ok {
		isOnion := strings.Index(domain, ".onion") == len(domain)-6
		rawurl = domain + "/.well-known/lnurlp/" + name
		if isOnion {
			rawurl = "http://" + rawurl
		} else {
			rawurl = "https://" + rawurl
		}
	} else if strings.HasPrefix(rawlnurl, "http") {
		rawurl = rawlnurl
	} else {
		foundUrl, ok := lnurl.FindLNURLInText(rawlnurl)
		if !ok {
			return "", nil,
				errors.New("invalid bech32-encoded lnurl: " + rawlnurl)
		}
		rawurl, err = lnurl.LNURLDecode(foundUrl)
		if err != nil {
			return "", nil, err
		}
	}

	parsed, err := url.Parse(rawurl)
	if err != nil {
		return rawurl, nil, err
	}

	query := parsed.Query()

	switch query.Get("tag") {
	case "login":
		value, err := lnurl.HandleAuth(rawurl, parsed, query)
		return rawurl, value, err
	case "withdrawRequest":
		if value, ok := lnurl.HandleFastWithdraw(query); ok {
			return rawurl, value, nil
		}
	}
	client, err := getHttpClient()
	if err != nil {
		return "", nil, err
	}

	resp, err := client.Get(rawurl)
	if err != nil {
		return rawurl, nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return rawurl, nil, err
	}

	j := gjson.ParseBytes(b)
	if j.Get("status").String() == "ERROR" {
		return rawurl, nil, lnurl.LNURLErrorResponse{
			URL:    parsed,
			Reason: j.Get("reason").String(),
			Status: "ERROR",
		}
	}

	switch j.Get("tag").String() {
	case "withdrawRequest":
		value, err := lnurl.HandleWithdraw(j)
		return rawurl, value, err
	case "payRequest":
		value, err := lnurl.HandlePay(j)
		return rawurl, value, err
	case "channelRequest":
		value, err := lnurl.HandleChannel(j)
		return rawurl, value, err
	default:
		return rawurl, nil, errors.New("unknown response tag " + j.String())
	}
}

func (bot *TipBot) sendToLightningAddress(ctx context.Context, m *tb.Message, address string, amount int) error {
	split := strings.Split(address, "@")
	if len(split) != 2 {
		return fmt.Errorf("lightning address format wrong")
	}
	host := strings.ToLower(split[1])
	name := strings.ToLower(split[0])

	// convert address scheme into LNURL Bech32 format
	callback := fmt.Sprintf("https://%s/.well-known/lnurlp/%s", host, name)

	log.Infof("[sendToLightningAddress] %s: callback: %s", GetUserStr(m.Sender), callback)

	lnurl, err := lnurl.LNURLEncode(callback)
	if err != nil {
		return err
	}

	if amount > 0 {
		m.Text = fmt.Sprintf("/lnurl %d %s", amount, lnurl)
	} else {
		m.Text = fmt.Sprintf("/lnurl %s", lnurl)
	}
	bot.lnurlHandler(ctx, m)
	return nil
}
