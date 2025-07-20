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

	// Setup handler
	h := handler.NewHandler(&configMutex, appConfig)

	// Start config file watching with callback to update handler
	go config.WatchConfigFile(&configMutex, &appConfig, func(newConfig *config.Config) {
		h.UpdateConfig(newConfig)
	})

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/", h.Handle)

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




