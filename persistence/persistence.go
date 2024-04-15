package persistence

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/gsp"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
)

const (
	GridStatesFileName     = "grid_states.json"
	BlacklistFileName      = "blacklist.json"
	MarkForRemovalFileName = "mark_for_removal.json"
)

type Registry struct {
	FileName string
	DataPtr  any
}

func getFullPath(fileName string) string {
	return config.TheConfig.DataFolder + "/" + fileName
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

var registries = []Registry{
	{FileName: GridStatesFileName, DataPtr: &gsp.TheGridEnv},
	{FileName: BlacklistFileName, DataPtr: &blacklist.TheBlacklist},
	{FileName: MarkForRemovalFileName, DataPtr: &gsp.ForRemoval},
}

func Init() {
	for _, r := range registries {
		if err := load(r.DataPtr, r.FileName); err != nil {
			discord.Errorf("Error loading %s: %v", r.FileName, err)
			panic(err)
		}
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		_ = <-sigChan
		for _, r := range registries {
			if err := save(r.DataPtr, r.FileName); err != nil {
				discord.Errorf("Error saving %s: %v", r.FileName, err)
				panic(err)
			}
		}
	}()
}
