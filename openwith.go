package main

import (
	"encoding/json"
	"fmt"
	"log"
	"openwith/config"
	"openwith/handler"
	"openwith/logger"
	"os"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func init() {
	Run = MainRun
}

var appConfig *config.Config
var configMutex sync.RWMutex

func MainRun() *echo.Echo {
	// Initialize logger first (check if running as service)
	serviceMode := os.Getenv("SERVICE_MODE") == "true"
	if err := logger.InitializeWithMode(serviceMode); err != nil {
		log.Printf("Failed to initialize logger: %v", err)
	}

	var err error
	appConfig, err = config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config file:", err)
	}

	go watchConfigFile()

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Setup handler
	h := &handler.Handler{
		GetConfig: func() *config.Config {
			configMutex.RLock()
			defer configMutex.RUnlock()
			return appConfig
		},
	}

	e.POST("/", h.OpenFile)

	port := ":44525"
	configMutex.RLock()
	if appConfig.Port != 0 {
		port = fmt.Sprintf(":%d", appConfig.Port)
	}
	configMutex.RUnlock()

	log.Printf("#########################################")
	log.Printf("#                                       #")
	log.Printf("#   Starting server on port %s...   #", port)
	log.Printf("#                                       #")
	log.Printf("#########################################")

	// Log configuration details as formatted JSON
	configJSON, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config to JSON: %v", err)
	} else {
		log.Printf("Config loaded successfully:")
		log.Printf("%s", string(configJSON))
	}

	e.Logger.Fatal(e.Start(port))

	return e
}




func watchConfigFile() {
	configPath, err := config.GetConfigPath()
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
					if newConfig, err := config.LoadConfig(); err == nil {
						configMutex.Lock()
						appConfig = newConfig
						configMutex.Unlock()
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
