package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type URLPattern struct {
	Pattern     string            `json:"pattern"`
	Args        []string          `json:"args"`
	URLParams   map[string]string `json:"url_params"`
	CompiledReg *regexp.Regexp    `json:"-"`
}

type Config struct {
	Application string       `json:"application"`
	Port        int          `json:"port"`
	URLPatterns []URLPattern `json:"url_patterns"`
}

func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	for i := range config.URLPatterns {
		pattern := &config.URLPatterns[i]
		pattern.CompiledReg, err = regexp.Compile(pattern.Pattern)
		if err != nil {
			return nil, err
		}
	}

	return &config, nil
}

func GetConfigPath() (string, error) {
	// Get the directory of the current executable
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "config.json"), nil
}

// ConfigUpdateCallback is the function type for config update callbacks
type ConfigUpdateCallback func(*Config)

// WatchConfigFile monitors config file changes and calls the callback when updated
func WatchConfigFile(configMutex *sync.RWMutex, appConfig **Config, callback ConfigUpdateCallback) {
	configPath, err := GetConfigPath()
	if err != nil {
		log.Printf("Failed to get config path: %v", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if stat, err := os.Stat(configPath); err == nil {
		lastModTime = stat.ModTime()
	}

	for {
		select {
		case <-ticker.C:
			if stat, err := os.Stat(configPath); err == nil {
				if stat.ModTime().After(lastModTime) {
					log.Println("Config file changed, reloading...")
					if newConfig, err := LoadConfig(); err == nil {
						configMutex.Lock()
						*appConfig = newConfig
						configMutex.Unlock()
						
						// Call the callback with the new config
						if callback != nil {
							callback(newConfig)
						}
						
						log.Println("Config reloaded successfully")
					} else {
						log.Printf("Failed to reload config: %v", err)
					}
					lastModTime = stat.ModTime()
				}
			}
		}
	}
}
