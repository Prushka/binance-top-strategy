package main

import (
	"encoding/json"
	"fmt"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

func PrintAsJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Error(err)
	}
	log.Info(string(b))
}

func DiscordWebhook(chat string) {
	if TheConfig.DiscordWebhook == "" {
		return
	}
	name := "BTS"
	message := discordwebhook.Message{
		Username: &name,
		Content:  &chat,
	}
	err := discordwebhook.SendMessage(TheConfig.DiscordWebhook, message)
	if err != nil {
		log.Errorf("error sending message to discord: %v", err)
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
