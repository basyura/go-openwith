package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
