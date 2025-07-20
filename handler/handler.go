package handler

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"openwith/config"
	"openwith/logger"
	"openwith/windows"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	configMutex *sync.RWMutex
	appConfig   *config.Config
}

// NewHandler creates a new handler instance
func NewHandler(configMutex *sync.RWMutex, appConfig *config.Config) *Handler {
	return &Handler{
		configMutex: configMutex,
		appConfig:   appConfig,
	}
}

// GetConfig safely returns the current configuration
func (h *Handler) GetConfig() *config.Config {
	h.configMutex.RLock()
	defer h.configMutex.RUnlock()
	return h.appConfig
}

// UpdateConfig safely updates the configuration
func (h *Handler) UpdateConfig(newConfig *config.Config) {
	h.configMutex.Lock()
	defer h.configMutex.Unlock()
	h.appConfig = newConfig
}

// Handle handles POST requests to open URLs with configured applications
func (h *Handler) Handle(c echo.Context) error {
	log.Println("-------------------------------------------------------")
	var body RequestBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
	}

	log.Println("url :", body.URL)
	if body.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "URL parameter is required"})
	}

	appConfig := h.GetConfig()
	args, modifiedURL := h.processURL(body.URL, appConfig)
	cmdArgs := h.buildCommandArgs(args, modifiedURL)

	if err := h.executeCommand(cmdArgs, appConfig); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Cannot start application: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message":     "URL opened successfully",
		"url":         body.URL,
		"application": appConfig.Application,
		"args":        fmt.Sprintf("%v", args),
	})
}

func (h *Handler) processURL(originalURL string, appConfig *config.Config) ([]string, string) {
	var args []string
	modifiedURL := originalURL

	for _, pattern := range appConfig.URLPatterns {
		if pattern.CompiledReg == nil {
			continue
		}
		if !pattern.CompiledReg.MatchString(originalURL) {
			continue
		}

		modifiedURL = h.modifyURLParams(originalURL, pattern.URLParams)
		args = h.buildArgs(pattern.Args, modifiedURL)
		break
	}

	return args, modifiedURL
}

func (h *Handler) modifyURLParams(originalURL string, urlParams map[string]string) string {
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

func (h *Handler) buildArgs(patternArgs []string, modifiedURL string) []string {
	args := make([]string, len(patternArgs))
	for i, arg := range patternArgs {
		args[i] = strings.Replace(arg, "$url", modifiedURL, -1)
	}
	return args
}

func (h *Handler) buildCommandArgs(args []string, modifiedURL string) []string {
	if len(args) > 0 {
		return args
	}
	return []string{modifiedURL}
}

func (h *Handler) executeCommand(cmdArgs []string, appConfig *config.Config) error {
	app := appConfig.Application

	// Check if running as service and try to execute in user session
	serviceMode := os.Getenv("SERVICE_MODE") == "true"
	if serviceMode && runtime.GOOS == "windows" {
		return windows.ExecuteCommandInUserSession(app, cmdArgs)
	}

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