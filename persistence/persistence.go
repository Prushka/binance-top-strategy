package persistence

import (
	"BinanceTopStrategies/config"
	"encoding/json"
	"os"
)

const (
	GridStatesFileName     = "grid_states.json"
	BlacklistFileName      = "blacklist.json"
	MarkForRemovalFileName = "mark_for_removal.json"
)

func getFullPath(fileName string) string {
	return config.TheConfig.DataFolder + "/" + fileName
}

func Save(t any, fileName string) error {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getFullPath(fileName), b, 0666)
}

func Load(dataPointer any, fileName string) error {
	if _, err := os.Stat(getFullPath(fileName)); os.IsNotExist(err) {
		return nil
	}
	b, err := os.ReadFile(getFullPath(fileName))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dataPointer)
}
