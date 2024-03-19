package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
)

func PrintAsJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Error(err)
	}
	log.Info(string(b))
}

func hourToSeconds(hour int) int {
	return hour * 3600
}

func dayToSeconds(day int) int {
	return day * 3600 * 24
}
