package simplelog

import (
	"encoding/json"
	"fmt"
	"strings"
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
		fmt.Printf("%s ", Color("INFO", BLUE))
	} else {
		fmt.Print("[INFO]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Errorf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", Color("ERROR", RED))
	} else {
		fmt.Print("[ERROR]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Debugf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", Color("DEBUG", PURPLE))
	} else {
		fmt.Print("[DEBUG]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

func Warnf(msg string, args ...interface{}) {
	if pretty {
		fmt.Printf("%s ", Color("WARN", YELLOW))
	} else {
		fmt.Print("[WARN]: ")
	}
	fmt.Printf(msg, args...)
	fmt.Println()
}

type Property struct {
	Foreground *RGB
	Background *RGB
	Italic     bool
	Bold       bool
	Underline  bool
}

type RGB struct {
	R, G, B int
}

// Simple color
func Color(msg, color string) string {
	return fmt.Sprintf("%s%s%s", color, msg, COLOR_OFF)
}

// Advanced color
func ColorA(msg string, prop Property) string {
	style := make([]string, 0)
	if prop.Bold {
		style = append(style, "1")
	}
	if prop.Italic {
		style = append(style, "3")
	}
	if prop.Underline {
		style = append(style, "4")
	}
	if prop.Foreground != nil {
		style = append(style, fmt.Sprintf("38;2;%d;%d;%d", prop.Foreground.R, prop.Foreground.G, prop.Foreground.B))
	}
	if prop.Background != nil {
		style = append(style, fmt.Sprintf("48;2;%d;%d;%d", prop.Background.R, prop.Background.G, prop.Background.B))
	}
	return fmt.Sprintf("\033[%sm%s%s", strings.Join(style, ";"), msg, COLOR_OFF)
}
