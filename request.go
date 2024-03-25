package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"time"
)

type BinanceBaseResponse struct {
	Code          string                 `json:"code"`
	Message       string                 `json:"message"`
	MessageDetail map[string]interface{} `json:"messageDetail"`
	Success       bool                   `json:"success"`
}

type BinanceResponse interface {
	getCode() string
}

func (b BinanceBaseResponse) getCode() string {
	return b.Code
}

func request[T any](url string, payload any, response T) (T, []byte, error) {
	queryJson, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json",
		bytes.NewBuffer(queryJson))
	if err != nil {
		return response, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, nil, err
	}
	err = json.Unmarshal(body, response)
	if err != nil {
		return response, nil, err
	}
	return response, body, nil
}

func privateRequest[T BinanceResponse](url, method string, payload any, response T) (T, []byte, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return response, nil, err
	}
	var r io.Reader
	if p != nil {
		r = bytes.NewBuffer(p)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return response, nil, err
	}
	req.Header.Set("Cookie", TheConfig.COOKIE)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("Clienttype", "web")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Csrftoken", TheConfig.CSRFToken)
	req.Header.Set("Sec-Ch-Ua", "\\\"Chromium\\\";v=\\\"122\\\", \\\"Not(A:Brand\\\";v=\\\"24\\\", \\\"Google Chrome\\\";v=\\\"122\\\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\\\"macOS\\\"")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return response, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, nil, err
	}
	log.Infof("Response: %s", body)
	time.Sleep(1 * time.Second)
	err = json.Unmarshal(body, response)
	if err == nil {
		if response.getCode() == "100002001" || response.getCode() == "100001005" {
			DiscordWebhook("Error, login expired")
			return response, body, fmt.Errorf("login expired")
		}
	}
	return response, body, err
}
