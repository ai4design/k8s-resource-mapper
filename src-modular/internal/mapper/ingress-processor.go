package mapper

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressProcessor handles Ingress resource processing
type IngressProcessor struct {
	client     *client.K8sClient
	namespace  string
	resources  []types.Resource
	relations  []types.Relationship
	mu         sync.RWMutex
	visualOpts *types.VisualOptions
}

// NewIngressProcessor creates a new Ingress processor
func NewIngressProcessor(client *client.K8sClient, namespace string, opts *types.VisualOptions) *IngressProcessor {
	return &IngressProcessor{
		client:     client,
		namespace:  namespace,
		visualOpts: opts,
		resources:  make([]types.Resource, 0),
		relations:  make([]types.Relationship, 0),
	}
}

// Process processes Ingress resources
func (p *IngressProcessor) Process(ctx context.Context) error {
	ingresses, err := p.client.Clientset.NetworkingV1().Ingresses(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Ingresses: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(ingresses.Items))

	for _, ing := range ingresses.Items {
		wg.Add(1)
		go func(i networkingv1.Ingress) {
			defer wg.Done()
			if err := p.processIngress(ctx, &i); err != nil {
				errChan <- err
			}
		}(ing)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("Ingress processing error: %v", err)
		}
	}

	return nil
}

// processIngress processes a single Ingress
func (p *IngressProcessor) processIngress(ctx context.Context, ing *networkingv1.Ingress) error {
	// Create Ingress resource
	ingResource := types.Resource{
		Type:      types.ResourceTypeIngress,
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Labels:    ing.Labels,
		Data:      ing,
		Status:    p.getIngressStatus(ing),
		Metrics:   p.getIngressMetrics(ing),
	}

	p.addResource(ingResource)

	// Process TLS configuration
	if err := p.processTLSSecrets(ctx, ing, ingResource); err != nil {
		return err
	}

	// Process backend services
	if err := p.processBackendServices(ctx, ing, ingResource); err != nil {
		return err
	}

	// Process ingress class if available
	if err := p.processIngressClass(ctx, ing, ingResource); err != nil {
		return err
	}

	return nil
}

// getIngressStatus returns the status of an Ingress
func (p *IngressProcessor) getIngressStatus(ing *networkingv1.Ingress) types.ResourceStatus {
	status := types.ResourceStatus{
		Phase: "Active",
		Ready: true,
	}

	// Check if TLS is configured
	if len(ing.Spec.TLS) > 0 {
		status.Details = "TLS Enabled"
	}

	// Check load balancer status
	if len(ing.Status.LoadBalancer.Ingress) > 0 {
		addresses := make([]string, 0)
		for _, ingress := range ing.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				addresses = append(addresses, ingress.IP)
			}
			if ingress.Hostname != "" {
				addresses = append(addresses, ingress.Hostname)
			}
		}
		if len(addresses) > 0 {
			status.Details += fmt.Sprintf(", LoadBalancer: %s", strings.Join(addresses, ", "))
		}
	}

	return status
}

// getIngressMetrics returns metrics for an Ingress
func (p *IngressProcessor) getIngressMetrics(ing *networkingv1.Ingress) types.ResourceMetrics {
	var rules, paths int
	for _, rule := range ing.Spec.Rules {
		rules++
		if rule.HTTP != nil {
			paths += len(rule.HTTP.Paths)
		}
	}

	return types.ResourceMetrics{
		Rules: rules,
		Paths: paths,
		TLS:   len(ing.Spec.TLS),
	}
}

// processTLSSecrets processes TLS secrets used by the Ingress
func (p *IngressProcessor) processTLSSecrets(ctx context.Context, ing *networkingv1.Ingress, ingResource types.Resource) error {
	for _, tls := range ing.Spec.TLS {
		if tls.SecretName == "" {
			continue
		}

		secret, err := p.client.Clientset.CoreV1().Secrets(ing.Namespace).Get(ctx, tls.SecretName, metav1.GetOptions{})
		if err != nil {
			continue // Skip if secret not found
		}

		secretResource := types.Resource{
			Type:      types.ResourceTypeSecret,
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Labels:    secret.Labels,
			Data:      secret,
			Status: types.ResourceStatus{
				Phase: "Active",
				Ready: true,
				Details: fmt.Sprintf("TLS secret for hosts: %s",
					strings.Join(tls.Hosts, ", ")),
			},
		}

		p.addResource(secretResource)
		p.addRelationship(types.Relationship{
			Source:      ingResource,
			Target:      secretResource,
			Type:        types.RelationshipTypeUses,
			Description: fmt.Sprintf("uses for TLS: %s",
				strings.Join(tls.Hosts, ", ")),
		})
	}

	return nil
}

// processBackendServices processes services referenced by the Ingress
func (p *IngressProcessor) processBackendServices(ctx context.Context, ing *networkingv1.Ingress, ingResource types.Resource) error {
	processedServices := make(map[string]bool)

	// Process default backend if specified
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		if err := p.processService(ctx, ing.Namespace, ing.Spec.DefaultBackend.Service, ingResource, "default backend"); err != nil {
			return err
		}
		processedServices[ing.Spec.DefaultBackend.Service.Name] = true
	}

	// Process rules
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil || processedServices[path.Backend.Service.Name] {
				continue
			}

			description := fmt.Sprintf("host: %s, path: %s", rule.Host, path.Path)
			if err := p.processService(ctx, ing.Namespace, path.Backend.Service, ingResource, description); err != nil {
				return err
			}
			processedServices[path.Backend.Service.Name] = true
		}
	}

	return nil
}

// processService processes a single backend service
func (p *IngressProcessor) processService(ctx context.Context, namespace string, backend *networkingv1.IngressServiceBackend, ingResource types.Resource, description string) error {
	svc, err := p.client.Clientset.CoreV1().Services(namespace).Get(ctx, backend.Name, metav1.GetOptions{})
	if err != nil {
		return nil // Skip if service not found
	}

	svcResource := types.Resource{
		Type:      types.ResourceTypeService,
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Labels:    svc.Labels,
		Data:      svc,
		Status: types.ResourceStatus{
			Phase:   "Active",
			Ready:   true,
			Details: fmt.Sprintf("Port: %d", backend.Port.Number),
		},
	}

	p.addResource(svcResource)
	p.addRelationship(types.Relationship{
		Source:      ingResource,
		Target:      svcResource,
		Type:        types.RelationshipTypeExposes,
		Description: description,
	})

	return nil
}

// processIngressClass processes the IngressClass if specified
func (p *IngressProcessor) processIngressClass(ctx context.Context, ing *networkingv1.Ingress, ingResource types.Resource) error {
	if ing.Spec.IngressClassName == nil {
		return nil
	}

	ingressClass, err := p.client.Clientset.NetworkingV1().IngressClasses().Get(ctx, *ing.Spec.IngressClassName, metav1.GetOptions{})
	if err != nil {
		return nil // Skip if ingress class not found
	}

	classResource := types.Resource{
		Type:      types.ResourceTypeIngressClass,
		Name:      ingressClass.Name,
		Namespace: "",
		Labels:    ingressClass.Labels,
		Data:      ingressClass,
		Status: types.ResourceStatus{
			Phase:   "Active",
			Ready:   true,
			Details: fmt.Sprintf("Controller: %s", ingressClass.Spec.Controller),
		},
	}

	p.addResource(classResource)
	p.addRelationship(types.Relationship{
		Source:      ingResource,
		Target:      classResource,
		Type:        types.RelationshipTypeUses,
		Description: "uses ingress class",
	})

	return nil
}

// Helper functions

func (p *IngressProcessor) addResource(resource types.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, resource)
}

func (p *IngressProcessor) addRelationship(rel types.Relationship) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relations = append(p.relations, rel)
}

// GetResources returns the processed resources
func (p *IngressProcessor) GetResources() []types.Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resources
}

// GetRelationships returns the processed relationships
func (p *IngressProcessor) GetRelationships() []types.Relationship {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.relations
}
