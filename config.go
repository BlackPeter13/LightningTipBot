package main

import "github.com/jinzhu/configor"

var Configuration = struct {
	AdminKey               string `json:"lnbits_admin_id" yaml:"lnbits_admin_id"`
	LnbitsKey              string `json:"lnbits_admin_key" yaml:"lnbits_admin_key"`
	LnbitsUrl              string `json:"lnbits_url" yaml:"lnbits_url"`
	WebhookServer          string `json:"webhook_server" yaml:"webhook_server"`
	ApiKey                 string `json:"telegram_api_key" yaml:"telegram_api_key"`
	DbPath                 string `json:"db_path" yaml:"db_path"`
	TransactionsPath       string `json:"transactions_path" yaml:"transactions_path"`
	MessageDisposeDuration int64  `json:"message_dispose_duration" yaml:"message_dispose_duration"`
}{}

func init() {
	err := configor.Load(&Configuration, "config.yaml")
	if err != nil {
		panic(err)
	}
}