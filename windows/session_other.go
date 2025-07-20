//go:build !windows

package windows

// Dummy implementations for non-Windows platforms
func ExecuteCommandInUserSession(command string, args []string) error {
	// Not supported on non-Windows platforms
	return nil
}