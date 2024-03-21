package main

import (
	"encoding/json"
	"fmt"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

func IntPointer(i int) *int {
	return &i
}

func PrintAsJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Error(err)
	}
	log.Info(string(b))
}

var discordMessageChan = make(chan string, 100)

func DiscordWebhook(chat string) {
	log.Info(chat)
	discordMessageChan <- chat
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

func DiscordSend(chat string) {
	if TheConfig.DiscordWebhook == "" {
		return
	}
	name := TheConfig.DiscordName
	message := discordwebhook.Message{
		Username: &name,
		Content:  &chat,
	}
	err := discordwebhook.SendMessage(TheConfig.DiscordWebhook, message)

	if err != nil {
		de := &DiscordError{}
		jsonErr := json.Unmarshal([]byte(err.Error()), de)
		if jsonErr != nil {
			log.Errorf("error sending message to discord: %v", err)
			return
		}
		if de.RetryAfter > 0 {
			time.Sleep(time.Duration(de.RetryAfter) * time.Second)
			DiscordSend(chat)
		}
	}
}

func getPublicIP() string {
	// The URL of the service that returns the public IP
	url := "http://api.ipify.org"

	// Making a GET request to the URL
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching IP address:", err)
		return ""
	}
	defer resp.Body.Close()

	// Reading the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}

	return string(body)
}
