package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type URLPattern struct {
	Pattern   string            `json:"pattern"`
	Args      []string          `json:"args"`
	URLParams map[string]string `json:"url_params"`
}

type Config struct {
	Application string       `json:"application"`
	URLPatterns []URLPattern `json:"url_patterns"`
}

var config Config

func loadConfig() error {
	file, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(&config)
}

type RequestBody struct {
	URL string `json:"url"`
}

func openFile(c echo.Context) error {
	fmt.Println("-------------------------------------------------------")
	var body RequestBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
	}

	fmt.Println("url :", body.URL)
	if body.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "URL parameter is required"})
	}

	var args []string
	modifiedURL := body.URL

	for _, pattern := range config.URLPatterns {
		matched, err := regexp.MatchString(pattern.Pattern, body.URL)
		if err != nil {
			continue
		}
		if !matched {
			continue
		}

		if len(pattern.URLParams) > 0 {
			parsedURL, err := url.Parse(body.URL)
			if err == nil {
				query := parsedURL.Query()
				for key, value := range pattern.URLParams {
					if query.Has(key) {
						query.Set(key, value)
					}
				}
				parsedURL.RawQuery = query.Encode()
				modifiedURL = parsedURL.String()
			}
		}

		args = make([]string, len(pattern.Args))
		for i, arg := range pattern.Args {
			args[i] = strings.Replace(arg, "$url", modifiedURL, -1)
		}
		break
	}

	var cmdArgs []string
	if len(args) > 0 {
		cmdArgs = args
	} else {
		cmdArgs = []string{modifiedURL}
	}

	cmd := exec.Command(config.Application, cmdArgs...)

	fmt.Printf("Executing command: %s %s\n", config.Application, strings.Join(cmdArgs, " "))

	err := cmd.Start()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Cannot start application: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message":     "URL opened successfully",
		"url":         body.URL,
		"application": config.Application,
		"args":        fmt.Sprintf("%v", args),
	})
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal("Failed to load config file:", err)
	}

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/", openFile)

	log.Println("Starting server on port 44525...")
	e.Logger.Fatal(e.Start(":44525"))
}
