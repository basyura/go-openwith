package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"openwith/config"
	"openwith/handler"
	"os/exec"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func init() {
	Run = mainRun
}

var appConfig *config.Config

func mainRun() *echo.Echo {
	var err error
	appConfig, err = config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config file:", err)
	}

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/", openFile)

	port := ":44525"
	if appConfig.Port != 0 {
		port = fmt.Sprintf(":%d", appConfig.Port)
	}
	log.Printf("Starting server on port %s...", port)
	e.Logger.Fatal(e.Start(port))

	return e
}

func openFile(c echo.Context) error {
	fmt.Println("-------------------------------------------------------")
	var body handler.RequestBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
	}

	fmt.Println("url :", body.URL)
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

	for _, pattern := range appConfig.URLPatterns {
		matched, err := regexp.MatchString(pattern.Pattern, originalURL)
		if err != nil {
			continue
		}
		if !matched {
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
	cmd := exec.Command(appConfig.Application, cmdArgs...)
	fmt.Printf("Executing command: %s %s\n", appConfig.Application, strings.Join(cmdArgs, " "))
	return cmd.Start()
}
