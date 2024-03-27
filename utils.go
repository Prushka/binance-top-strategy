package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net/http"
	"time"
)

var timing = time.Now()

func Time(s string) {
	Discordf("*%s took: %v*", s, time.Since(timing))
	timing = time.Now()
}

func ResetTime() {
	timing = time.Now()
}

func IntPointer(i int) *int {
	return &i
}

func StringPointer(s string) *string {
	return &s
}

func Float64Pointer(f float64) *float64 {
	return &f
}

func Int64Pointer(i int64) *int64 {
	return &i
}

func generateRandomNumberUUID() string {
	const charset = "0123456789"
	b := make([]byte, 19)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func asJson(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Error(err)
	}
	return string(b)
}

func getPublicIP() string {
	url := "http://api.ipify.org"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching IP address:", err)
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}

	return string(body)
}
