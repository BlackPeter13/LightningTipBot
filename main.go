package main

import (
	"fmt"
)

func main() {
	defer withRecovery()
	bot := NewBot()
	bot.Start()
}

func withRecovery() {
	if r := recover(); r != nil {
		fmt.Println("Recovered panic: ", r)
	}
}
