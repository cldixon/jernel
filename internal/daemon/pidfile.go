package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/cldixon/jernel/internal/config"
)

// PIDPath returns the path to the daemon PID file
func PIDPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.pid"), nil
}

// WritePID writes the current process ID to the PID file
func WritePID() error {
	path, err := PIDPath()
	if err != nil {
		return err
	}

	pid := os.Getpid()
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// ReadPID reads the PID from the PID file
func ReadPID() (int, error) {
	path, err := PIDPath()
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// RemovePID removes the PID file
func RemovePID() error {
	path, err := PIDPath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsRunning checks if a daemon process is currently running
func IsRunning() (bool, int, error) {
	pid, err := ReadPID()
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0, nil
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, clean up stale PID file
		_ = RemovePID()
		return false, 0, nil
	}

	return true, pid, nil
}

// StopRunning sends SIGTERM to the running daemon
func StopRunning() error {
	pid, err := ReadPID()
	if err != nil {
		return fmt.Errorf("failed to read PID: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	return process.Signal(syscall.SIGTERM)
}
