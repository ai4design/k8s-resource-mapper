package utils

import (
	"fmt"
	"os"
	"regexp"
)

// ANSI color codes
const (
	ColorRed     = "\033[0;31m"
	ColorGreen   = "\033[0;32m"
	ColorYellow  = "\033[1;33m"
	ColorBlue    = "\033[0;34m"
	ColorMagenta = "\033[0;35m"
	ColorCyan    = "\033[0;36m"
	ColorGray    = "\033[0;37m"
	ColorReset   = "\033[0m"

	// Bold colors
	ColorBoldRed     = "\033[1;31m"
	ColorBoldGreen   = "\033[1;32m"
	ColorBoldYellow  = "\033[1;33m"
	ColorBoldBlue    = "\033[1;34m"
	ColorBoldMagenta = "\033[1;35m"
	ColorBoldCyan    = "\033[1;36m"
	ColorBoldGray    = "\033[1;37m"
)

// Colorize returns a string wrapped with color codes
func Colorize(color string, text string) string {
	return color + text + ColorReset
}

// ColorizedPrintf returns a formatted string with color
func ColorizedPrintf(color string, format string, a ...interface{}) string {
	return Sprintf(color+format+ColorReset, a...)
}

// ResourceColors maps Kubernetes resource types to colors
var ResourceColors = map[string]string{
	"Namespace":  ColorCyan,
	"Pod":        ColorGreen,
	"Service":    ColorBlue,
	"Ingress":    ColorMagenta,
	"ConfigMap":  ColorYellow,
	"Deployment": ColorBoldBlue,
	"HPA":        ColorBoldGreen,
	"Secret":     ColorBoldRed,
	"Default":    ColorGray,
}

// GetResourceColor returns the color for a given resource type
func GetResourceColor(resourceType string) string {
	if color, exists := ResourceColors[resourceType]; exists {
		return color
	}
	return ResourceColors["Default"]
}

// DisableColors can be set to true to disable color output
var DisableColors bool

// InitColors determines if colors should be disabled based on environment
func InitColors() {
	// Disable colors if output is not a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		DisableColors = true
	}

	// Disable colors if NO_COLOR environment variable is set
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		DisableColors = true
	}
}

// Sprintf provides a safe sprintf that handles disabled colors
func Sprintf(format string, a ...interface{}) string {
	if DisableColors {
		// Strip color codes when colors are disabled
		format = stripColorCodes(format)
	}
	return fmt.Sprintf(format, a...)
}

// stripColorCodes removes ANSI color codes from a string
func stripColorCodes(s string) string {
	// Regular expression to match ANSI escape codes
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(s, "")
}
