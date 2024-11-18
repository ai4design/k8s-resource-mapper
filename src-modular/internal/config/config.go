package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Namespace     string
	ExcludeNs     StringSliceFlag
	KubeConfig    string
	VisualOptions *VisualOptions
}

// VisualOptions holds visualization-related configuration
type VisualOptions struct {
	ShowColors   bool
	ShowDetails  bool
	CompactView  bool
	Format       string      // text, json, yaml
	GroupBy      string      // namespace, type, none
	MaxDepth     int         // Maximum relationship depth to show
	FocusOn      StringSlice // Resource types to focus on
	HideTypes    StringSlice // Resource types to hide
	CustomColors ColorScheme // Custom color definitions
}

// ColorScheme defines custom colors for different elements
type ColorScheme struct {
	ResourceTypes map[string]string
	Relationships map[string]string
	Status        map[string]string
	Symbols       map[string]string
}

// StringSliceFlag implements flag.Value interface for string slice flags
type StringSliceFlag []string

func (s *StringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *StringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Contains checks if a namespace is in the exclusion list
func (s *StringSliceFlag) Contains(namespace string) bool {
	for _, ns := range *s {
		if ns == namespace {
			return true
		}
	}
	return false
}

// StringSlice is a simple string slice type
type StringSlice []string

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Namespace:  "",
		ExcludeNs:  []string{},
		KubeConfig: "",
		VisualOptions: &VisualOptions{
			ShowColors:  true,
			ShowDetails: true,
			CompactView: false,
			Format:      "text",
			GroupBy:     "namespace",
			MaxDepth:    5,
			FocusOn:     []string{},
			HideTypes:   []string{},
			CustomColors: ColorScheme{
				ResourceTypes: DefaultResourceColors(),
				Relationships: DefaultRelationshipColors(),
				Status:        DefaultStatusColors(),
				Symbols:       DefaultSymbols(),
			},
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check kubeconfig
	if c.KubeConfig != "" {
		if _, err := os.Stat(c.KubeConfig); err != nil {
			return fmt.Errorf("kubeconfig file not found: %s", c.KubeConfig)
		}
	}

	// Validate namespace
	if c.Namespace != "" {
		// Namespace name validation rules
		if !IsValidNamespaceName(c.Namespace) {
			return fmt.Errorf("invalid namespace name: %s", c.Namespace)
		}
	}

	// Initialize VisualOptions if not set
	if c.VisualOptions == nil {
		c.VisualOptions = DefaultConfig().VisualOptions
	}

	// Validate output format
	validFormats := map[string]bool{"text": true, "json": true, "yaml": true}
	if !validFormats[c.VisualOptions.Format] {
		return fmt.Errorf("invalid output format: %s", c.VisualOptions.Format)
	}

	// Validate grouping
	validGroupings := map[string]bool{"namespace": true, "type": true, "none": true}
	if !validGroupings[c.VisualOptions.GroupBy] {
		return fmt.Errorf("invalid grouping: %s", c.VisualOptions.GroupBy)
	}

	// Validate max depth
	if c.VisualOptions.MaxDepth < 1 {
		return fmt.Errorf("invalid max depth: %d (must be >= 1)", c.VisualOptions.MaxDepth)
	}

	return nil
}

// IsValidNamespaceName checks if a namespace name is valid
func IsValidNamespaceName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// DNS-1123 label validation rules
	validName := true
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-') {
			validName = false
			break
		}
	}

	return validName
}

// DefaultResourceColors returns the default color scheme for resources
func DefaultResourceColors() map[string]string {
	return map[string]string{
		"Namespace":  "\033[0;36m", // Cyan
		"Pod":        "\033[0;32m", // Green
		"Service":    "\033[0;34m", // Blue
		"Ingress":    "\033[0;35m", // Magenta
		"ConfigMap":  "\033[0;33m", // Yellow
		"Deployment": "\033[1;34m", // Bold Blue
		"HPA":        "\033[1;32m", // Bold Green
		"Secret":     "\033[1;31m", // Bold Red
		"Default":    "\033[0;37m", // Gray
	}
}

// DefaultRelationshipColors returns the default color scheme for relationships
func DefaultRelationshipColors() map[string]string {
	return map[string]string{
		"owns":     "\033[0;32m", // Green
		"uses":     "\033[0;34m", // Blue
		"exposes":  "\033[0;35m", // Magenta
		"targets":  "\033[0;36m", // Cyan
		"provides": "\033[0;33m", // Yellow
		"default":  "\033[0;37m", // Gray
	}
}

// DefaultStatusColors returns the default color scheme for status indicators
func DefaultStatusColors() map[string]string {
	return map[string]string{
		"Running":   "\033[0;32m", // Green
		"Pending":   "\033[0;33m", // Yellow
		"Failed":    "\033[0;31m", // Red
		"Succeeded": "\033[0;32m", // Green
		"Unknown":   "\033[0;37m", // Gray
		"Warning":   "\033[0;33m", // Yellow
		"Error":     "\033[0;31m", // Red
		"Info":      "\033[0;34m", // Blue
	}
}

// DefaultSymbols returns the default symbols used in visualization
func DefaultSymbols() map[string]string {
	return map[string]string{
		"arrow":      "➜",
		"dot":        "●",
		"success":    "✓",
		"warning":    "⚠",
		"error":      "✗",
		"info":       "ℹ",
		"tree":       "├──",
		"treeLast":   "└──",
		"treeVert":   "│",
		"autoscale":  "⟳",
		"lock":       "🔒",
		"connection": "⇄",
	}
}
