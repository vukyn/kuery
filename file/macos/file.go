package file

const dot_character = 46

// IsHidden returns true if the file or directory is hidden
func IsHidden(path string) (bool, error) {
	return path[0] == dot_character, nil
}
