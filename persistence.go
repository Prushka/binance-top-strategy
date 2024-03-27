package main

import (
	"encoding/json"
	"os"
)

const (
	GridStatesFileName = "grid_states.json"
)

func getFullPath(fileName string) string {
	return TheConfig.DataFolder + "/" + fileName
}

func save(t any, fileName string) error {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getFullPath(fileName), b, 0666)
}

func load(dataPointer any, fileName string) error {
	if _, err := os.Stat(getFullPath(fileName)); os.IsNotExist(err) {
		return nil
	}
	b, err := os.ReadFile(getFullPath(fileName))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dataPointer)
}
