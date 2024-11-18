package mapper

import (
	"context"
	"fmt"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/config"
	"k8s-resource-mapper/internal/types"
	"k8s-resource-mapper/internal/utils"
	"k8s-resource-mapper/internal/visualizer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceMapper handles the mapping of Kubernetes resources
type ResourceMapper struct {
	client    *client.K8sClient
	config    *config.Config
	ctx       context.Context
	cancel    context.CancelFunc
	resources types.ResourceMapping
	mu        sync.RWMutex
}

// NewResourceMapper creates a new ResourceMapper instance
func NewResourceMapper(cfg *config.Config) (*ResourceMapper, error) {
	k8sClient, err := client.NewK8sClient(cfg.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ResourceMapper{
		client:    k8sClient,
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		resources: types.ResourceMapping{},
	}, nil
}

// Process starts the resource mapping process
func (rm *ResourceMapper) Process() error {
	// Get namespaces to process
	namespaces, err := rm.getNamespaces()
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %v", err)
	}

	// Process each namespace
	for _, ns := range namespaces {
		if err := rm.processNamespace(ns); err != nil {
			utils.PrintWarning(fmt.Sprintf("Error processing namespace %s: %v", ns, err))
			continue
		}
	}

	// Visualize the results
	if err := rm.Visualize(); err != nil {
		return fmt.Errorf("visualization error: %v", err)
	}

	return nil
}

// Visualize renders the resource mapping visualization
func (rm *ResourceMapper) Visualize() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Create the visualizer with current resource mapping
	viz := visualizer.NewVisualizer(rm.resources)

	// Apply visualization options from config
	if rm.config.VisualOptions != nil {
		viz.SetOptions(
			rm.config.VisualOptions.ShowDetails,
			rm.config.VisualOptions.ShowColors,
		)
	}

	// Render and print the visualization
	output := viz.RenderClusterView()
	fmt.Println(output)

	return nil
}

// getNamespaces returns the list of namespaces to process
func (rm *ResourceMapper) getNamespaces() ([]string, error) {
	if rm.config.Namespace != "" {
		// Verify namespace exists
		_, err := rm.client.Clientset.CoreV1().Namespaces().Get(
			rm.ctx,
			rm.config.Namespace,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("namespace %s not found: %v", rm.config.Namespace, err)
		}
		return []string{rm.config.Namespace}, nil
	}

	nsList, err := rm.client.Clientset.CoreV1().Namespaces().List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, ns := range nsList.Items {
		if !rm.config.ExcludeNs.Contains(ns.Name) {
			namespaces = append(namespaces, ns.Name)
		}
	}

	return namespaces, nil
}

// processNamespace processes resources in a single namespace
func (rm *ResourceMapper) processNamespace(namespace string) error {
	utils.PrintLine()
	fmt.Printf("%s\n", utils.FormatResource("Namespace", namespace))
	utils.PrintLine()

	// Create processors for different resource types
	processors := []types.ResourceProcessor{
		NewDeploymentProcessor(rm.client, namespace),
		NewServiceProcessor(rm.client, namespace),
		NewConfigMapProcessor(rm.client, namespace),
		NewIngressProcessor(rm.client, namespace),
	}

	// Process resources concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, len(processors))

	for _, processor := range processors {
		wg.Add(1)
		go func(p types.ResourceProcessor) {
			defer wg.Done()
			if err := p.Process(rm.ctx, namespace); err != nil {
				errCh <- err
				return
			}
			rm.addRelationships(p.GetRelationships())
		}(processor)
	}

	// Wait for all processors to complete
	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			return fmt.Errorf("processor error: %v", err)
		}
	}

	return nil
}

// addRelationships adds relationships to the resource mapping
func (rm *ResourceMapper) addRelationships(relationships []types.Relationship) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources.Relationships = append(rm.resources.Relationships, relationships...)
}

// Cleanup performs cleanup operations
func (rm *ResourceMapper) Cleanup() {
	rm.cancel()
}

// GetResourceMapping returns the current resource mapping
func (rm *ResourceMapper) GetResourceMapping() types.ResourceMapping {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.resources
}
