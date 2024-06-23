package utils

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func OverwriteQuote(symbol, quote string, currencyLength int) string {
	return symbol[:len(symbol)-currencyLength] + quote
}

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

func FormatPair(symbol string) string {
	return symbol
}

func MapValues[T comparable, U any](m map[T]U) []U {
	values := make([]U, 0)
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func TillNextRefresh() time.Duration {
	minutesTillNextHour := 60 - time.Now().Minute()
	if minutesTillNextHour >= 30 {
		return time.Duration(minutesTillNextHour+config.TheConfig.ShiftMinutesAfterHour) * time.Minute
	}
	return time.Duration(minutesTillNextHour+config.TheConfig.ShiftMinutesAfterHour+60) * time.Minute
}

func IntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func IntMax(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func ParseFloatPointer(s string) (*float64, error) {
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
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

func InRange(what, reference, pct float64) bool {
	return what > reference*(1-pct) && what < reference*(1+pct)
}

func MinTime(times ...time.Time) time.Time {
	m := times[0]
	for _, t := range times {
		if !t.IsZero() && (t.Before(m) || m.IsZero()) {
			m = t
		}
	}
	return m
}
