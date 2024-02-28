package file

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// CreateFilePath creates a file path if not exist.
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
//	Example:
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
func ClearFiles(filepaths ...string) error {
	for _, filepath := range filepaths {
		if err := os.Remove(filepath); err != nil {
			return err
		}
	}
	return nil
}

// ClearFolders removes multiple directories.
func ClearFolders(dirpaths ...string) error {
	for _, dirpath := range dirpaths {
		if err := os.RemoveAll(dirpath); err != nil {
			return err
		}
	}
	return nil
}

// CopyFile copies a file from src to dst.
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
//	Mode:
//	- 0: Before
//	- 1: After
//	Example:
//	- AppendFile([]byte("Hello World"), "temp/output.txt", 2)
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

// // ZipSingleFile zip the sourceFile path to outputFile path
// func ZipSingleFile(sourceFile, outputFile string) error {
// 	if err := CreateFilePath(outputFile); err != nil {
// 		return err
// 	}

// 	archive, err := os.Create(outputFile)
// 	if err != nil {
// 		return err
// 	}
// 	defer archive.Close()

// 	zipWriter := zip.NewWriter(archive)
// 	defer zipWriter.Close()

// 	f, err := os.Open(sourceFile)
// 	if err != nil {
// 		return err
// 	}
// 	defer f.Close()

// 	w, err := zipWriter.Create(path.Base(f.Name()))
// 	if err != nil {
// 		return err
// 	}
// 	if _, err := io.Copy(w, f); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // ZipMultipleFile zip list of sourceFile path to outputFile path
// func ZipMultipleFile(outputFile string, sourceFiles ...string) error {
// 	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
// 	file, err := os.OpenFile(outputFile, flags, 0644)
// 	if err != nil {
// 		return fmt.Errorf("failed to open zip for writing: %s", err)
// 	}
// 	defer file.Close()

// 	zipw := zip.NewWriter(file)
// 	defer zipw.Close()

// 	for _, filename := range sourceFiles {
// 		if err := appendFiles(filename, zipw); err != nil {
// 			return fmt.Errorf("failed to add file %s to zip: %s", filename, err)
// 		}
// 	}
// 	return err
// }

// func appendFiles(filename string, zipw *zip.Writer) error {
// 	file, err := os.Open(filename)
// 	if err != nil {
// 		return fmt.Errorf("failed to open %s: %s", filename, err)
// 	}
// 	defer file.Close()

// 	_, filename = filepath.Split(filename)
// 	wr, err := zipw.Create(filename)
// 	if err != nil {
// 		msg := "failed to create entry for %s in zip file: %s"
// 		return fmt.Errorf(msg, filename, err)
// 	}

// 	if _, err := io.Copy(wr, file); err != nil {
// 		return fmt.Errorf("failed to write %s to zip: %s", filename, err)
// 	}

// 	return nil
// }

// func ZZip(folder string, files ...string) error {
// 	targetZip := folder //"output.zip"

// 	// Mở file zip để ghi
// 	zipFile, err := os.Create(targetZip)
// 	if err != nil {
// 		return err
// 	}
// 	defer zipFile.Close()

// 	// Tạo biến zip.Writer để ghi vào file zip
// 	zipWriter := zip.NewWriter(zipFile)
// 	defer zipWriter.Close()
// 	for _, file := range files {
// 		err = write(zipWriter, file)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func write(zipWriter *zip.Writer, file string) error {
// 	fileInfo, err := os.Stat(file)
// 	if err != nil {
// 		return err
// 	}

// 	// Tạo một header cho file trong file zip
// 	header, err := zip.FileInfoHeader(fileInfo)
// 	if err != nil {
// 		return err
// 	}

// 	// Đặt tên file trong file zip
// 	header.Name = fileInfo.Name()

// 	// Tạo một đối tượng io.Writer để ghi dữ liệu vào file zip
// 	writer, err := zipWriter.CreateHeader(header)
// 	if err != nil {
// 		return err
// 	}

// 	// Mở file nguồn để đọc dữ liệu
// 	sourceFile, err := os.Open(file)
// 	if err != nil {
// 		return err
// 	}
// 	defer sourceFile.Close()

// 	// Sử dụng buffer để đọc dữ liệu từ file nguồn và ghi vào file zip
// 	buffer := make([]byte, 1024)
// 	for {
// 		bytesRead, err := sourceFile.Read(buffer)
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return err
// 		}
// 		_, err = writer.Write(buffer[:bytesRead])
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
