package utils

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func ShortDur(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func TillNextRefresh() time.Duration {
	minutesTillNextHour := 60 - time.Now().Minute()
	if minutesTillNextHour >= 30 {
		return time.Duration(minutesTillNextHour+config.TheConfig.ShiftMinutesAfterHour) * time.Minute
	}
	return time.Duration(minutesTillNextHour+config.TheConfig.ShiftMinutesAfterHour+60) * time.Minute
}

var timing = time.Now()

func Time(s string) {
	discord.Infof("*%s took: %v*", s, time.Since(timing))
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

func GenerateRandomNumberUUID() string {
	const charset = "0123456789"
	b := make([]byte, 19)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func AsJson(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		discord.Errorf("Error marshalling json: %v", err)
	}
	return string(b)
}

func GetPublicIP() string {
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
