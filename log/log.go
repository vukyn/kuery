package log

import (
	"encoding/json"
	"fmt"
)

// Print with formatted
func PrettyPrint(v interface{}) string {
	res, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(res)
}

var pretty = false

func SetPrettyLog() {
	pretty = true
}

func DisablePrettyLog() {
	pretty = false
}

func Infof(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", style("INFO", BLUE))
	} else {
		fmt.Print("[INFO]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Errorf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", style("ERROR", RED))
	} else {
		fmt.Print("[ERROR]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Debugf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", style("DEBUG", PURPLE))
	} else {
		fmt.Print("[DEBUG]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Warnf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", style("WARN", YELLOW))
	} else {
		fmt.Print("[WARN]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Color(msg, color string) string {
	return style(msg, color)
}

func style(msg, color string) string {
	return fmt.Sprintf("%s%s%s", color, msg, COLOR_OFF)
}
