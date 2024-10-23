//go:build windows

package main

import (
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

const dot_character = '.'

func IsHidden(path string) (bool, error) {
	switch runtime.GOOS {
	case "windows":
		if path[0] == dot_character {
			return true, nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return false, err
		}

		pointer, err := windows.UTF16PtrFromString(absPath)
		if err != nil {
			return false, err
		}
		attributes, err := windows.GetFileAttributes(pointer)
		if err != nil {
			return false, err
		}
		return attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0, nil
	default:
		return path[0] == dot_character, nil
	}
}
