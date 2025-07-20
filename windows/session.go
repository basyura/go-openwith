//go:build windows

package windows

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

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

// ExecuteCommandInUserSession executes a command in the active user session
func ExecuteCommandInUserSession(app string, cmdArgs []string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("user session execution is only supported on Windows")
	}

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