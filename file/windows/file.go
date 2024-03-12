package file

import (
	"path/filepath"
	"runtime"
	"syscall"
)

const dot_character = 46

// IsHidden returns true if the file or directory is hidden
func IsHidden(path string) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		// dotfiles also count as hidden
		if path[0] == dot_character {
			return true, nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return false, err
		}

		pointer, err := syscall.UTF16PtrFromString(absPath)
		if err != nil {
			return false, err
		}
		attributes, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			return false, err
		}
		return attributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0, nil
	default:
		return path[0] == dot_character, nil
	}
}
