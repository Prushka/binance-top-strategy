package discord

import (
	"BinanceTopStrategies/config"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type messagePayload struct {
	Content     string `json:"content"`
	WebhookType int    `json:"webhookType"`
}

var messages = make(map[int][]string)
var mutex sync.RWMutex

func Json(chat string) string {
	return "```json\n" + chat + "\n```"
}

func Actionf(f string, args ...any) {
	Webhooks(format(f, args...), ActionWebhook, DefaultWebhook)
}

func Infof(f string, args ...any) {
	Webhooks(format(f, args...), DefaultWebhook)
}

func Errorf(f string, args ...any) {
	Webhooks(format(f, args...), ErrorWebhook, DefaultWebhook)
}

func Blacklistf(f string, args ...any) {
	Webhooks(format(f, args...), BlacklistWebhook, DefaultWebhook)
}

func Orderf(f string, args ...any) {
	Webhooks(format(f, args...), OrderWebhook)
}

func format(f string, args ...any) string {
	s := f
	if len(args) > 0 {
		s = fmt.Sprintf(f, args...)
	}
	return s
}

func Webhooks(chat string, webhookTypes ...int) {
	hooks := mapset.NewSet(webhookTypes...)
	if hooks.Contains(ErrorWebhook) {
		log.Error(chat)
	} else {
		log.Info(chat)
	}
	mutex.Lock()
	defer mutex.Unlock()
	for _, webhookType := range webhookTypes {
		messages[webhookType] = append(messages[webhookType], chat)
	}
}

func Init() {
	scheduler := gocron.NewScheduler(time.Now().Location())
	_, err := scheduler.SingletonMode().Every(5).Seconds().Do(func() {
		mutex.Lock()
		currentMessages := make(map[int][]string)
		for k, v := range messages {
			currentMessages[k] = v
		}
		clear(messages)
		mutex.Unlock()
		for webhookType, messages := range currentMessages {
			if len(messages) > 0 {
				chunks := make([]string, 0)
				for _, message := range messages {
					if len(chunks) == 0 {
						chunks = append(chunks, message)
						continue
					}
					if len(chunks[len(chunks)-1])+len(message) > 1500 {
						chunks = append(chunks, message)
					} else {
						chunks[len(chunks)-1] = chunks[len(chunks)-1] + "\n" + message
					}
				}
				for _, chunk := range chunks {
					send(messagePayload{Content: chunk, WebhookType: webhookType})
				}
			}
		}
	})
	if err != nil {
		log.Fatalf("error scheduling discord service: %v", err)
	}
	scheduler.StartAsync()
}

type errorResponse struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
}

const (
	DefaultWebhook int = iota
	ActionWebhook
	OrderWebhook
	ErrorWebhook
	BlacklistWebhook
)

func send(payload messagePayload) {
	if config.TheConfig.DiscordWebhook == "" {
		return
	}
	name := config.TheConfig.DiscordName
	message := discordwebhook.Message{
		Username: &name,
		Content:  &payload.Content,
	}
	var err error
	switch payload.WebhookType {
	case ActionWebhook:
		if config.TheConfig.DiscordWebhookAction == "" {
			return
		}
		err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhookAction, message)
	case OrderWebhook:
		if config.TheConfig.DiscordWebhookOrder == "" {
			return
		}
		err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhookOrder, message)
	case ErrorWebhook:
		if config.TheConfig.DiscordWebhookError == "" {
			return
		}
		err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhookError, message)
	case BlacklistWebhook:
		if config.TheConfig.DiscordWebhookBlacklist == "" {
			return
		}
		err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhookBlacklist, message)
	default:
		err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhook, message)
	}

	if err != nil {
		de := &errorResponse{}
		jsonErr := json.Unmarshal([]byte(err.Error()), de)
		if jsonErr != nil {
			log.Errorf("error sending message to discord: %v", err)
			return
		}
		if de.RetryAfter > 0 {
			time.Sleep(time.Duration(de.RetryAfter) * time.Second)
			send(payload)
		} else {
			log.Errorf("error sending message to discord: %v", err)
		}
	}
}
