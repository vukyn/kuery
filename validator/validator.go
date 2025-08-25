package validator

import (
	"regexp"
	"strconv"

	"github.com/asaskevich/govalidator"
)

// snake_case: only lowercase letters, numbers, and underscores, no leading or trailing underscores, no consecutive underscores
func IsSnakeCase(str string) bool {
	var snakeCaseRegex = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
	return snakeCaseRegex.MatchString(str)
}

func StringLength(str string, min int, max int) bool {
	return govalidator.ByteLength(str, strconv.Itoa(min), strconv.Itoa(max))
}

func IsURL(str string) bool {
	return govalidator.IsURL(str)
}

func IsEmail(str string) bool {
	return govalidator.IsEmail(str)
}

func IsUUID(str string) bool {
	return govalidator.IsUUID(str)
}

func IsUUIDV4(str string) bool {
	return govalidator.IsUUIDv4(str)
}
