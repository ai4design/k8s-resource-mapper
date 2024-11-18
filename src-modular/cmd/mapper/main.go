package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s-resource-mapper/internal/config"
	"k8s-resource-mapper/internal/mapper"
	"k8s-resource-mapper/internal/utils"
)

// CLI flags
type flags struct {
	namespace    string
	excludeNs    config.StringSliceFlag
	help         bool
	kubeconfig   string
	noColor      bool
	noDetails    bool
	compactView  bool
	outputFormat string
}

func parseFlags() *flags {
	f := &flags{}

	// Resource selection flags
	flag.StringVar(&f.namespace, "n", "", "Process only the specified namespace")
	flag.StringVar(&f.namespace, "namespace", "", "Process only the specified namespace (alternative)")
	flag.Var(&f.excludeNs, "exclude-ns", "Exclude specified namespaces")
	flag.StringVar(&f.kubeconfig, "kubeconfig", "", "Path to kubeconfig file")

	// Visualization flags
	flag.BoolVar(&f.noColor, "no-color", false, "Disable color output")
	flag.BoolVar(&f.noDetails, "no-details", false, "Show minimal resource details")
	flag.BoolVar(&f.compactView, "compact", false, "Use compact visualization mode")
	flag.StringVar(&f.outputFormat, "output", "text", "Output format (text, json, yaml)")

	// Help flag
	flag.BoolVar(&f.help, "h", false, "Show help message")
	flag.BoolVar(&f.help, "help", false, "Show help message (alternative)")

	// Parse flags
	flag.Parse()

	return f
}

func main() {
	// Parse command line flags
	flags := parseFlags()

	// Show help if requested
	if flags.help {
		printHelp()
		os.Exit(0)
	}

	// Create configuration
	cfg := &config.Config{
		Namespace:  flags.namespace,
		ExcludeNs:  flags.excludeNs,
		KubeConfig: flags.kubeconfig,
		VisualOptions: &config.VisualOptions{
			ShowColors:  !flags.noColor,
			ShowDetails: !flags.noDetails,
			CompactView: flags.compactView,
			Format:      flags.outputFormat,
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Printf("%sConfiguration error: %v%s\n",
			utils.ColorRed, err, utils.ColorReset)
		os.Exit(1)
	}

	// Initialize the resource mapper
	rm, err := mapper.NewResourceMapper(cfg)
	if err != nil {
		fmt.Printf("%sError initializing resource mapper: %v%s\n",
			utils.ColorRed, err, utils.ColorReset)
		os.Exit(1)
	}

	// Setup graceful shutdown
	setupSignalHandler(rm)

	// Print header
	printHeader()

	// Process resources
	if err := rm.Process(); err != nil {
		fmt.Printf("%sError processing resources: %v%s\n",
			utils.ColorRed, err, utils.ColorReset)
		os.Exit(1)
	}

	fmt.Printf("\n%sResource mapping complete!%s\n",
		utils.ColorGreen, utils.ColorReset)
}

func setupSignalHandler(rm *mapper.ResourceMapper) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Printf("\n%sReceived shutdown signal, cleaning up...%s\n",
			utils.ColorYellow, utils.ColorReset)
		rm.Cleanup()
		os.Exit(0)
	}()
}

func printHeader() {
	fmt.Printf("%sKubernetes Resource Mapper%s\n",
		utils.ColorGreen, utils.ColorReset)
	utils.PrintLine()
}

func printHelp() {
	fmt.Printf(`Kubernetes Resource Mapper - Visualize cluster resource relationships

Usage:
  %s [options]

Resource Selection Options:
  -n, --namespace string     Process only the specified namespace
  --exclude-ns string       Exclude specified namespaces (can be specified multiple times)
  --kubeconfig string      Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)

Visualization Options:
  --no-color               Disable colored output
  --no-details            Show minimal resource details
  --compact               Use compact visualization mode
  --output string         Output format: text, json, yaml (default: text)

Other Options:
  -h, --help              Show this help message

Examples:
  # Show all namespaces
  %s

  # Show specific namespace
  %s -n default

  # Exclude system namespaces
  %s --exclude-ns kube-system --exclude-ns kube-public

  # Compact view without colors
  %s --compact --no-color

  # JSON output
  %s --output json

For more information and examples, visit:
https://github.com/yourusername/k8s-resource-mapper
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func init() {
	// Initialize colors based on terminal capabilities
	utils.InitColors()
}
