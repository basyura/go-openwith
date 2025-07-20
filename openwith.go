package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"openwith/config"
	"openwith/handler"
	"openwith/logger"
	"os"
	"os/exec"
	"strings"
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

	e.POST("/", openFile)

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

func openFile(c echo.Context) error {
	log.Println("-------------------------------------------------------")
	var body handler.RequestBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
	}

	log.Println("url :", body.URL)
	if body.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "URL parameter is required"})
	}

	args, modifiedURL := processURL(body.URL)
	cmdArgs := buildCommandArgs(args, modifiedURL)

	if err := executeCommand(cmdArgs); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Cannot start application: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message":     "URL opened successfully",
		"url":         body.URL,
		"application": appConfig.Application,
		"args":        fmt.Sprintf("%v", args),
	})
}

func processURL(originalURL string) ([]string, string) {
	var args []string
	modifiedURL := originalURL

	configMutex.RLock()
	defer configMutex.RUnlock()

	for _, pattern := range appConfig.URLPatterns {
		if pattern.CompiledReg == nil {
			continue
		}
		if !pattern.CompiledReg.MatchString(originalURL) {
			continue
		}

		modifiedURL = modifyURLParams(originalURL, pattern.URLParams)
		args = buildArgs(pattern.Args, modifiedURL)
		break
	}

	return args, modifiedURL
}

func modifyURLParams(originalURL string, urlParams map[string]string) string {
	if len(urlParams) == 0 {
		return originalURL
	}

	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	query := parsedURL.Query()
	for key, value := range urlParams {
		if query.Has(key) {
			query.Set(key, value)
		}
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func buildArgs(patternArgs []string, modifiedURL string) []string {
	args := make([]string, len(patternArgs))
	for i, arg := range patternArgs {
		args[i] = strings.Replace(arg, "$url", modifiedURL, -1)
	}
	return args
}

func buildCommandArgs(args []string, modifiedURL string) []string {
	if len(args) > 0 {
		return args
	}
	return []string{modifiedURL}
}

func executeCommand(cmdArgs []string) error {
	configMutex.RLock()
	app := appConfig.Application
	configMutex.RUnlock()

	cmd := exec.Command(app, cmdArgs...)
	log.Printf("Executing command: %s %s\n", app, strings.Join(cmdArgs, " "))
	
	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	
	// Convert output to UTF-8 if needed
	outputStr := logger.ConvertToUTF8(output)
	
	// Log execution result
	if err != nil {
		log.Printf("Command execution failed: %v", err)
		if len(outputStr) > 0 {
			log.Printf("Command output: %s", outputStr)
		}
		return err
	} else {
		log.Printf("Command executed successfully")
		if len(outputStr) > 0 {
			log.Printf("Command output: %s", outputStr)
		}
	}
	
	return nil
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
