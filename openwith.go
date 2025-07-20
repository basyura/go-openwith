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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

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

	// Check if running as service and try to execute in user session
	serviceMode := os.Getenv("SERVICE_MODE") == "true"
	if serviceMode && runtime.GOOS == "windows" {
		return executeCommandInUserSession(app, cmdArgs)
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

// Windows API functions for user session execution
var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	wtsapi32                  = syscall.NewLazyDLL("wtsapi32.dll")
	userenv                   = syscall.NewLazyDLL("userenv.dll")
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	
	procCloseHandle           = kernel32.NewProc("CloseHandle")
	procWTSEnumerateSessions  = wtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSQuerySessionInfo   = wtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSQueryUserToken     = wtsapi32.NewProc("WTSQueryUserToken")
	procWTSFreeMemory         = wtsapi32.NewProc("WTSFreeMemory")
	procCreateEnvironmentBlock = userenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock = userenv.NewProc("DestroyEnvironmentBlock")
	procOpenProcessToken      = advapi32.NewProc("OpenProcessToken")
	procDuplicateTokenEx      = advapi32.NewProc("DuplicateTokenEx")
	procCreateProcessAsUser   = advapi32.NewProc("CreateProcessAsUserW")
	procWaitForSingleObject   = kernel32.NewProc("WaitForSingleObject")
	procGetExitCodeProcess    = kernel32.NewProc("GetExitCodeProcess")
)

const (
	WTS_CURRENT_SERVER_HANDLE = 0
	WTSActive                 = 0
	WTSUserName               = 5
	TOKEN_DUPLICATE           = 0x0002
	TOKEN_ASSIGN_PRIMARY      = 0x0001
	TOKEN_QUERY               = 0x0008
	TOKEN_IMPERSONATE         = 0x0004
	TOKEN_ALL_ACCESS          = 0x000F01FF
	GENERIC_ALL_ACCESS        = 0x10000000
	SecurityImpersonation     = 2
	TokenPrimary              = 1
	STARTF_USESHOWWINDOW      = 0x00000001
	SW_HIDE                   = 0
	SW_SHOW                   = 5
	INFINITE                  = 0xFFFFFFFF
	WAIT_OBJECT_0             = 0x00000000
	CREATE_UNICODE_ENVIRONMENT = 0x00000400
	CREATE_NEW_CONSOLE        = 0x00000010
)

type WTS_SESSION_INFO struct {
	SessionId      uint32
	WinStationName *uint16
	State          uint32
}

type STARTUPINFO struct {
	Cb              uint32
	_               *uint16
	Desktop         *uint16
	Title           *uint16
	X               uint32
	Y               uint32
	XSize           uint32
	YSize           uint32
	XCountChars     uint32
	YCountChars     uint32
	FillAttribute   uint32
	Flags           uint32
	ShowWindow      uint16
	_               uint16
	_               *byte
	StdInput        syscall.Handle
	StdOutput       syscall.Handle
	StdError        syscall.Handle
}

type PROCESS_INFORMATION struct {
	Process   syscall.Handle
	Thread    syscall.Handle
	ProcessId uint32
	ThreadId  uint32
}

func executeCommandInUserSession(app string, cmdArgs []string) error {
	log.Printf("Executing command in user session: %s %s", app, strings.Join(cmdArgs, " "))
	
	// Get active session ID
	sessionId, err := getActiveSessionId()
	if err != nil {
		log.Printf("Failed to get active session: %v", err)
		return err
	}
	
	if sessionId == 0xFFFFFFFF {
		log.Printf("No active user session found")
		return fmt.Errorf("no active user session found")
	}
	
	log.Printf("Found active session ID: %d", sessionId)
	
	// Build command line
	cmdLine := fmt.Sprintf(`"%s" %s`, app, strings.Join(cmdArgs, " "))
	cmdLinePtr, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		return err
	}
	
	// Execute in user session
	err = createProcessInSession(sessionId, cmdLinePtr)
	if err != nil {
		log.Printf("Failed to create process in session: %v", err)
		return err
	}
	
	log.Printf("Command executed successfully in user session")
	return nil
}

func getActiveSessionId() (uint32, error) {
	var sessionInfo *WTS_SESSION_INFO
	var count uint32
	
	ret, _, _ := procWTSEnumerateSessions.Call(
		WTS_CURRENT_SERVER_HANDLE,
		0,
		1,
		uintptr(unsafe.Pointer(&sessionInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	
	if ret == 0 {
		return 0xFFFFFFFF, fmt.Errorf("WTSEnumerateSessions failed")
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(sessionInfo)))
	
	// Find active session
	sessions := (*[1024]WTS_SESSION_INFO)(unsafe.Pointer(sessionInfo))[:count:count]
	for _, session := range sessions {
		if session.State == WTSActive {
			return session.SessionId, nil
		}
	}
	
	return 0xFFFFFFFF, fmt.Errorf("no active session found")
}

func createProcessInSession(sessionId uint32, cmdLine *uint16) error {
	// Get user token for the session directly from WTS
	var userToken syscall.Handle
	ret, _, lastErr := procWTSQueryUserToken.Call(
		uintptr(sessionId),
		uintptr(unsafe.Pointer(&userToken)),
	)
	if ret == 0 {
		log.Printf("WTSQueryUserToken failed for session %d: %v", sessionId, lastErr)
		return fmt.Errorf("WTSQueryUserToken failed for session %d: %v", sessionId, lastErr)
	}
	defer procCloseHandle.Call(uintptr(userToken))
	
	log.Printf("Successfully obtained user token for session %d", sessionId)
	
	// Duplicate the token to create a primary token
	var primaryToken syscall.Handle
	ret, _, lastErr = procDuplicateTokenEx.Call(
		uintptr(userToken),
		TOKEN_ALL_ACCESS,
		0,
		SecurityImpersonation,
		TokenPrimary,
		uintptr(unsafe.Pointer(&primaryToken)),
	)
	if ret == 0 {
		log.Printf("DuplicateTokenEx failed: %v", lastErr)
		return fmt.Errorf("DuplicateTokenEx failed: %v", lastErr)
	}
	defer procCloseHandle.Call(uintptr(primaryToken))
	
	log.Printf("Successfully duplicated token to primary token")
	
	// Create environment block for the user
	var envBlock uintptr
	ret, _, lastErr = procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(primaryToken),
		0,
	)
	if ret == 0 {
		log.Printf("CreateEnvironmentBlock failed: %v", lastErr)
		return fmt.Errorf("CreateEnvironmentBlock failed: %v", lastErr)
	}
	defer procDestroyEnvironmentBlock.Call(envBlock)
	
	log.Printf("Successfully created environment block")
	
	// Setup startup info for user session
	desktop, _ := syscall.UTF16PtrFromString("winsta0\\default")
	startupInfo := STARTUPINFO{
		Cb:         uint32(unsafe.Sizeof(STARTUPINFO{})),
		Desktop:    desktop,
		Flags:      STARTF_USESHOWWINDOW,
		ShowWindow: SW_SHOW, // Show window in user session
	}
	
	var processInfo PROCESS_INFORMATION
	
	// Create process in user session with proper flags
	ret, _, lastErr = procCreateProcessAsUser.Call(
		uintptr(primaryToken),
		0,
		uintptr(unsafe.Pointer(cmdLine)),
		0,
		0,
		0,
		CREATE_UNICODE_ENVIRONMENT|CREATE_NEW_CONSOLE,
		envBlock,
		0,
		uintptr(unsafe.Pointer(&startupInfo)),
		uintptr(unsafe.Pointer(&processInfo)),
	)
	
	if ret == 0 {
		log.Printf("CreateProcessAsUser failed: %v", lastErr)
		return fmt.Errorf("CreateProcessAsUser failed: %v", lastErr)
	}
	
	log.Printf("Successfully created process in user session - Process ID: %d", processInfo.ProcessId)
	
	// Don't wait for GUI applications - just start them and return
	// Close handles immediately
	procCloseHandle.Call(uintptr(processInfo.Process))
	procCloseHandle.Call(uintptr(processInfo.Thread))
	
	log.Printf("Process handles closed, application should be running in session %d", sessionId)
	
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
