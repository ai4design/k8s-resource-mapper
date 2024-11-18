package mapper

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceProcessor handles service resource processing
type ServiceProcessor struct {
	client     *client.K8sClient
	namespace  string
	resources  []types.Resource
	relations  []types.Relationship
	mu         sync.RWMutex
	visualOpts *types.VisualOptions
}

// NewServiceProcessor creates a new service processor
func NewServiceProcessor(client *client.K8sClient, namespace string, opts *types.VisualOptions) *ServiceProcessor {
	return &ServiceProcessor{
		client:     client,
		namespace:  namespace,
		visualOpts: opts,
		resources:  make([]types.Resource, 0),
		relations:  make([]types.Relationship, 0),
	}
}

// Process processes service resources
func (p *ServiceProcessor) Process(ctx context.Context) error {
	services, err := p.client.Clientset.CoreV1().Services(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(services.Items))

	for _, svc := range services.Items {
		wg.Add(1)
		go func(s corev1.Service) {
			defer wg.Done()
			if err := p.processService(ctx, &s); err != nil {
				errChan <- err
			}
		}(svc)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("service processing error: %v", err)
		}
	}

	return nil
}

// processService processes a single service
func (p *ServiceProcessor) processService(ctx context.Context, svc *corev1.Service) error {
	// Create service resource
	svcResource := types.Resource{
		Type:      types.ResourceTypeService,
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Labels:    svc.Labels,
		Data:      svc,
		Status:    p.getServiceStatus(svc),
		Metrics:   p.getServiceMetrics(svc),
	}

	p.addResource(svcResource)

	// Process related resources concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 3) // pods, endpoints, ingresses

	// Process pods
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processPods(ctx, svc, svcResource); err != nil {
			errChan <- err
		}
	}()

	// Process endpoints
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processEndpoints(ctx, svc, svcResource); err != nil {
			errChan <- err
		}
	}()

	// Process ingresses
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processIngresses(ctx, svc, svcResource); err != nil {
			errChan <- err
		}
	}()

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// getServiceStatus returns the status of a service
func (p *ServiceProcessor) getServiceStatus(svc *corev1.Service) types.ResourceStatus {
	status := types.ResourceStatus{
		Phase: "Active",
		Ready: true,
	}

	// Add details based on service type
	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			status.Details = fmt.Sprintf("LoadBalancer: %s", getLoadBalancerAddress(svc))
		} else {
			status.Phase = "Pending"
			status.Ready = false
			status.Details = "Waiting for LoadBalancer"
		}
	case corev1.ServiceTypeNodePort:
		ports := getNodePorts(svc)
		status.Details = fmt.Sprintf("NodePorts: %s", ports)
	case corev1.ServiceTypeClusterIP:
		status.Details = fmt.Sprintf("ClusterIP: %s", svc.Spec.ClusterIP)
	}

	return status
}

// getServiceMetrics returns metrics for a service
func (p *ServiceProcessor) getServiceMetrics(svc *corev1.Service) types.ResourceMetrics {
	return types.ResourceMetrics{
		Ports: len(svc.Spec.Ports),
	}
}

// processPods processes pods related to a service
func (p *ServiceProcessor) processPods(ctx context.Context, svc *corev1.Service, svcResource types.Resource) error {
	if svc.Spec.Selector == nil {
		return nil // No selector, no pods to process
	}

	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{
		MatchLabels: svc.Spec.Selector,
	})

	pods, err := p.client.Clientset.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	for _, pod := range pods.Items {
		podResource := types.Resource{
			Type:      types.ResourceTypePod,
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
			Data:      pod,
			Status: types.ResourceStatus{
				Phase: string(pod.Status.Phase),
				Ready: isPodReady(&pod),
			},
		}

		p.addResource(podResource)
		p.addRelationship(types.Relationship{
			Source:      svcResource,
			Target:      podResource,
			Type:        types.RelationshipTypeTargets,
			Description: fmt.Sprintf("routes traffic to pod: %s", formatPorts(svc.Spec.Ports)),
		})
	}

	return nil
}

// processEndpoints processes endpoints related to a service
func (p *ServiceProcessor) processEndpoints(ctx context.Context, svc *corev1.Service, svcResource types.Resource) error {
	endpoints, err := p.client.Clientset.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	if err != nil {
		return nil // Skip if endpoints not found
	}

	// Add endpoints information to service status
	var activeEndpoints int
	for _, subset := range endpoints.Subsets {
		activeEndpoints += len(subset.Addresses)
	}

	svcResource.Status.Details += fmt.Sprintf(", Endpoints: %d active", activeEndpoints)
	return nil
}

// processIngresses processes ingresses related to a service
func (p *ServiceProcessor) processIngresses(ctx context.Context, svc *corev1.Service, svcResource types.Resource) error {
	ingresses, err := p.client.Clientset.NetworkingV1().Ingresses(svc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ingresses: %v", err)
	}

	for _, ing := range ingresses.Items {
		if isServiceReferencedByIngress(&ing, svc.Name) {
			ingResource := types.Resource{
				Type:      types.ResourceTypeIngress,
				Name:      ing.Name,
				Namespace: ing.Namespace,
				Labels:    ing.Labels,
				Data:      ing,
				Status: types.ResourceStatus{
					Phase: "Active",
					Ready: true,
					Details: fmt.Sprintf("Hosts: %s",
						formatIngressHosts(&ing)),
				},
			}

			p.addResource(ingResource)
			p.addRelationship(types.Relationship{
				Source:      ingResource,
				Target:      svcResource,
				Type:        types.RelationshipTypeExposes,
				Description: fmt.Sprintf("exposes via %s",
					formatIngressPaths(&ing, svc.Name)),
			})
		}
	}

	return nil
}

// Helper functions

func (p *ServiceProcessor) addResource(resource types.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, resource)
}

func (p *ServiceProcessor) addRelationship(rel types.Relationship) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relations = append(p.relations, rel)
}

func getLoadBalancerAddress(svc *corev1.Service) string {
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		ing := svc.Status.LoadBalancer.Ingress[0]
		if ing.IP != "" {
			return ing.IP
		}
		if ing.Hostname != "" {
			return ing.Hostname
		}
	}
	return "pending"
}

func getNodePorts(svc *corev1.Service) string {
	var ports []string
	for _, port := range svc.Spec.Ports {
		if port.NodePort > 0 {
			ports = append(ports, fmt.Sprintf("%d", port.NodePort))
		}
	}
	return strings.Join(ports, ", ")
}

func formatPorts(ports []corev1.ServicePort) string {
	var portStr []string
	for _, port := range ports {
		portStr = append(portStr, fmt.Sprintf("%d→%d/%s",
			port.Port, port.TargetPort.IntVal, port.Protocol))
	}
	return strings.Join(portStr, ", ")
}

func isServiceReferencedByIngress(ing *networkingv1.Ingress, serviceName string) bool {
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil &&
					path.Backend.Service.Name == serviceName {
					return true
				}
			}
		}
	}
	return false
}

func formatIngressHosts(ing *networkingv1.Ingress) string {
	var hosts []string
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	return strings.Join(hosts, ", ")
}

func formatIngressPaths(ing *networkingv1.Ingress, serviceName string) string {
	var paths []string
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil &&
					path.Backend.Service.Name == serviceName {
					paths = append(paths, fmt.Sprintf("%s%s",
						rule.Host, path.Path))
				}
			}
		}
	}
	return strings.Join(paths, ", ")
}

// GetResources returns the processed resources
func (p *ServiceProcessor) GetResources() []types.Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resources
}

// GetRelationships returns the processed relationships
func (p *ServiceProcessor) GetRelationships() []types.Relationship {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.relations
}
