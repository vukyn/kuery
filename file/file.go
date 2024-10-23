package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateFilePath(filePath string) error {
	path, _ := filepath.Split(filePath)
	if len(path) == 0 {
		return nil
	}

	_, err := os.Stat(path)
	if err != nil || os.IsExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
	}
	return err
}

// Write to a file.
// Create if not exist, or overwrite the existing file.
//
// Example:
//
//	WriteFile([]byte("Hello World"), "temp/output.txt")
func WriteFile(data []byte, filePath string) error {
	dir, _ := filepath.Split(filePath)

	if _, err := os.Stat(dir); err == nil {
		os.Remove(filePath)
	} else {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}
	return nil
}

// ClearFiles removes multiple files.
//
// Example:
//
//	ClearFiles("temp/output.txt", "temp/output2.txt")
func ClearFiles(filepaths ...string) error {
	for _, filepath := range filepaths {
		if err := os.Remove(filepath); err != nil {
			return err
		}
	}
	return nil
}

// ClearFolders removes multiple directories.
//
// Example:
//
//	ClearFolders("temp", "temp2")
func ClearFolders(dirpaths ...string) error {
	for _, dirpath := range dirpaths {
		if err := os.RemoveAll(dirpath); err != nil {
			return err
		}
	}
	return nil
}

// CopyFile copies a file from src to dst.
//
// Example:
//
//	CopyFile("temp/output.txt", "temp/output2.txt")
func CopyFile(srcPath, dstPath string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return err
	}
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if err := WriteFile(src, dstPath); err != nil {
		return err
	}
	return nil
}

// Append to a file.
// Create if not exist, or append to existing file.
//
// Mode:
//
//	0: Before
//	1: After
//
// Example:
//   - AppendFile([]byte("Hello World"), "temp/output.txt", 2)
func AppendFile(data []byte, filePath string, mode int) error {
	dir, _ := filepath.Split(filePath)

	if _, err := os.Stat(dir); err == nil {
		switch mode {
		case 0:
			f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			content = append(data, content...)
			if _, err := f.WriteAt(content, 0); err != nil {
				return err
			}
		case 1:
			f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := f.Write(data); err != nil {
				return err
			}
		}
		return nil
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}
	return nil
}

// GetAbsDir returns the absolute path of a directory
//
// Example:
//
//	GetAbsDir("temp")
func GetAbsDir(dirpath string) (string, error) {
	f, err := os.Stat(dirpath)
	if err != nil {
		return "", err
	}
	if !f.IsDir() {
		return "", fmt.Errorf("not a directory")
	}
	abs, err := filepath.Abs(dirpath)
	if err != nil {
		return "", err
	}
	return abs, nil
}
