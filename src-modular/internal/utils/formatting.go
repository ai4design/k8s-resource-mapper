package utils

import (
	"fmt"
	"strings"
)

const (
	LineWidth   = 80
	ArrowSymbol = ">"
	TreeSymbol  = "├──"
	LastSymbol  = "└──"
	VertSymbol  = "│"
)

// PrintLine prints a horizontal line
func PrintLine() {
	fmt.Println(strings.Repeat("-", LineWidth))
}

// CreateArrow creates an ASCII arrow of specified length
func CreateArrow(length int) string {
	return strings.Repeat("-", length) + ArrowSymbol
}

// TreePrefix returns the appropriate tree symbol based on position
func TreePrefix(isLast bool) string {
	if isLast {
		return LastSymbol
	}
	return TreeSymbol
}

// IndentLine indents a line with proper tree structure
func IndentLine(level int, isLast bool, text string) string {
	prefix := strings.Repeat("    ", level)
	if level > 0 {
		prefix = prefix + TreePrefix(isLast) + " "
	}
	return prefix + text
}

// FormatResource formats a resource name with its type
func FormatResource(resourceType, name string) string {
	return Colorize(GetResourceColor(resourceType), fmt.Sprintf("%s: %s", resourceType, name))
}

// FormatList formats a list of items with proper indentation and tree structure
func FormatList(items []string, level int) []string {
	var result []string
	for i, item := range items {
		isLast := i == len(items)-1
		result = append(result, IndentLine(level, isLast, item))
	}
	return result
}

// FormatError formats an error message with proper color
func FormatError(err error) string {
	return Colorize(ColorRed, fmt.Sprintf("Error: %v", err))
}

// FormatSuccess formats a success message with proper color
func FormatSuccess(msg string) string {
	return Colorize(ColorGreen, msg)
}

// FormatWarning formats a warning message with proper color
func FormatWarning(msg string) string {
	return Colorize(ColorYellow, msg)
}
