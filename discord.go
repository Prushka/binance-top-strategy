package main

import (
	"encoding/json"
	"fmt"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type DiscordMessagePayload struct {
	Content     string `json:"content"`
	WebhookType int    `json:"webhookType"`
}

var discordMessages = make(map[int][]string)
var discordMessagesMutex sync.RWMutex

func DiscordJson(chat string) string {
	return "```json\n" + chat + "\n```"
}

func Discordf(format string, args ...any) {
	s := format
	if len(args) > 0 {
		s = fmt.Sprintf(format, args...)
	}
	DiscordWebhookS(s, DefaultWebhook)
}

func DiscordWebhookS(chat string, webhookTypes ...int) {
	log.Info(chat)
	discordMessagesMutex.Lock()
	defer discordMessagesMutex.Unlock()
	for _, webhookType := range webhookTypes {
		discordMessages[webhookType] = append(discordMessages[webhookType], chat)
	}
}

func DiscordService() {
	_, err := scheduler.SingletonMode().Every(5).Seconds().Do(func() {
		discordMessagesMutex.Lock()
		currentMessages := make(map[int][]string)
		for k, v := range discordMessages {
			currentMessages[k] = v
		}
		clear(discordMessages)
		discordMessagesMutex.Unlock()
		for webhookType, messages := range currentMessages {
			if len(messages) > 0 {
				chunks := make([]string, 0)
				for _, message := range messages {
					if len(chunks) == 0 {
						chunks = append(chunks, message)
						continue
					}
					if len(chunks[len(chunks)-1])+len(message) > 2000 {
						chunks = append(chunks, message)
					} else {
						chunks[len(chunks)-1] = chunks[len(chunks)-1] + "\n" + message
					}
				}
				for _, chunk := range chunks {
					DiscordSend(DiscordMessagePayload{Content: chunk, WebhookType: webhookType})
				}
			}
		}
	})
	if err != nil {
		log.Fatalf("error scheduling discord service: %v", err)
	}
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
		} else {
			log.Errorf("error sending message to discord: %v", err)
		}
	}
}
