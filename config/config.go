package config

import (
	"encoding/json"
	"os"
)

type URLPattern struct {
	Pattern   string            `json:"pattern"`
	Args      []string          `json:"args"`
	URLParams map[string]string `json:"url_params"`
}

type Config struct {
	Application string       `json:"application"`
	Port        int          `json:"port"`
	URLPatterns []URLPattern `json:"url_patterns"`
}

func LoadConfig() (*Config, error) {
	file, err := os.Open("config.json")
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

	return &config, nil
}