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

const (
	CreateGridTooFrequently = "90805176"
)

type BinanceBaseResponse struct {
	Code          any                    `json:"code"`
	Message       string                 `json:"message"`
	MessageDetail map[string]interface{} `json:"messageDetail"`
	Success       bool                   `json:"success"`
}

type BinanceResponse interface {
	code() string
	success() bool
	message() string
	messageDetail() map[string]interface{}
}

func (b BinanceBaseResponse) code() string {
	return fmt.Sprintf("%v", b.Code)
}

func (b BinanceBaseResponse) success() bool {
	return b.Success
}

func (b BinanceBaseResponse) message() string {
	return b.Message
}

func (b BinanceBaseResponse) messageDetail() map[string]interface{} {
	return b.MessageDetail
}

func request[T BinanceResponse](url string, payload any, response T) (T, []byte, error) {
	return _request(url, "POST", 0, payload, nil, response)
}

func privateRequest[T BinanceResponse](url, method string, payload any, response T) (T, []byte, error) {
	headers := map[string]string{
		"Clienttype":         "web",
		"Cookie":             TheConfig.COOKIE,
		"Csrftoken":          TheConfig.CSRFToken,
		"Accept":             "*/*",
		"Accept-Language":    "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		"Sec-Ch-Ua":          "\\\"Chromium\\\";v=\\\"122\\\", \\\"Not(A:Brand\\\";v=\\\"24\\\", \\\"Google Chrome\\\";v=\\\"122\\\"",
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": "\\\"macOS\\\"",
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
		"User-Agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
	return _request(url, method, 0, payload, headers, response)
}

func _request[T BinanceResponse](url, method string, sleep time.Duration,
	payload any, headers map[string]string, response T) (T, []byte, error) {
	var p []byte
	var err error
	switch v := payload.(type) {
	case string:
		p = []byte(v)
	default:
		if payload != nil {
			p, err = json.Marshal(payload)
			if err != nil {
				return response, nil, err
			}
		}
	}
	var r io.Reader
	if p != nil {
		r = bytes.NewBuffer(p)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return response, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return response, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, nil, err
	}
	log.Debugf("Response: %s", body)
	time.Sleep(sleep)
	err = json.Unmarshal(body, response)
	if err != nil {
		log.Errorf("Response: %s", body)
		return response, body, err
	}
	if response.code() == "100002001" || response.code() == "100001005" {
		log.Errorf("Response: %s", body)
		Discordf("Error, login expired")
		return response, body, fmt.Errorf("login expired")
	}
	if !response.success() {
		log.Errorf("Response: %s", body)
		Discordf(response.message())
		return response, body, fmt.Errorf("error: %s", response.message())
	}
	return response, body, err
}
