package main

import (
	"encoding/json"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type DiscordMessagePayload struct {
	Content     string `json:"content"`
	WebhookType int    `json:"webhookType"`
}

var discordMessageChan = make(chan DiscordMessagePayload, 100)

func DiscordWebhook(chat string) {
	if strings.Contains(chat, "Opened") || strings.Contains(chat, "Cancelled") {
		DiscordWebhookS(chat, ActionWebhook)
	}
	DiscordWebhookS(chat, DefaultWebhook)
}

func DiscordWebhookS(chat string, webhookType int) {
	log.Info(chat)
	discordMessageChan <- DiscordMessagePayload{Content: chat, WebhookType: webhookType}
}

func DiscordService() {
	go func() {
		for {
			select {
			case chat := <-discordMessageChan:
				DiscordSend(chat)
			}
		}
	}()
}

type DiscordError struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
}

const (
	DefaultWebhook int = iota
	ActionWebhook
	OrderWebhook
)

func DiscordSend(payload DiscordMessagePayload) {
	if TheConfig.DiscordWebhook == "" {
		return
	}
	name := TheConfig.DiscordName
	message := discordwebhook.Message{
		Username: &name,
		Content:  &payload.Content,
	}
	var err error
	switch payload.WebhookType {
	case ActionWebhook:
		if TheConfig.DiscordWebhookAction == "" {
			return
		}
		err = discordwebhook.SendMessage(TheConfig.DiscordWebhookAction, message)
	case OrderWebhook:
		if TheConfig.DiscordWebhookOrder == "" {
			return
		}
		err = discordwebhook.SendMessage(TheConfig.DiscordWebhookOrder, message)
	default:
		err = discordwebhook.SendMessage(TheConfig.DiscordWebhook, message)
	}

	if err != nil {
		de := &DiscordError{}
		jsonErr := json.Unmarshal([]byte(err.Error()), de)
		if jsonErr != nil {
			log.Errorf("error sending message to discord: %v", err)
			return
		}
		if de.RetryAfter > 0 {
			time.Sleep(time.Duration(de.RetryAfter) * time.Second)
			DiscordSend(payload)
		}
	}
}