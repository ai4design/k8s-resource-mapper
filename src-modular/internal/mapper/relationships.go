package mapper

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"
	"k8s-resource-mapper/internal/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RelationshipProcessor handles relationship discovery and visualization
type RelationshipProcessor struct {
	client     *client.K8sClient
	namespace  string
	resources  []types.Resource
	relations  []types.Relationship
	processors []types.ResourceProcessor
	mu        sync.RWMutex
}

func NewRelationshipProcessor(client *client.K8sClient, namespace string) *RelationshipProcessor {
	return &RelationshipProcessor{
		client:    client,
		namespace: namespace,
		processors: []types.ResourceProcessor{
			NewDeploymentProcessor(client, namespace),
			NewServiceProcessor(client, namespace),
			NewConfigMapProcessor(client, namespace),
		},
	}
}

func (p *RelationshipProcessor) Process(ctx context.Context, namespace string) error {
	// Process resources concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, len(p.processors))

	for _, processor := range p.processors {
		wg.Add(1)
		go func(proc types.ResourceProcessor) {
			defer wg.Done()
			if err := proc.Process(ctx, namespace); err != nil {
				errCh <- err
				return
			}
			p.addRelationships(proc.GetRelationships())
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

	// Process ingress relationships last (they depend on service relationships)
	if err := p.processIngressRelationships(ctx, namespace); err != nil {
		return fmt.Errorf("error processing ingress relationships: %v", err)
	}

	return nil
}

func (p *RelationshipProcessor) GetRelationships() []types.Relationship {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.relations
}

func (p *RelationshipProcessor) addRelationships(relationships []types.Relationship) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relations = append(p.relations, relationships...)
}

func (p *RelationshipProcessor) processIngressRelationships(ctx context.Context, namespace string) error {
	ingresses, err := p.client.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ingresses: %v", err)
	}

	for _, ing := range ingresses.Items {
		ingResource := types.Resource{
			Type:      types.ResourceTypeIngress,
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Labels:    ing.Labels,
			Data:      ing,
		}

		p.resources = append(p.resources, ingResource)

		// Process each ingress rule
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					svcName := path.Backend.Service.Name
					svc, err := p.client.Clientset.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
					if err != nil {
						continue
					}

					svcResource := types.Resource{
						Type:      types.ResourceTypeService,
						Name:      svc.Name,
						Namespace: svc.Namespace,
						Labels:    svc.Labels,
						Data:      svc,
					}

					p.relations = append(p.relations, types.Relationship{
						Source:      ingResource,
						Target:      svcResource,
						Type:        types.RelationshipTypeExposes,
						Description: fmt.Sprintf("exposes via %s%s", rule.Host, path.Path),
					})
				}
			}
		}
	}

	return nil
}

// visualizeRelationships generates a visual representation of relationships
func (p *RelationshipProcessor) VisualizeRelationships() string {
	var result strings.Builder

	// Group relationships by source type
	groupedRelations := make(map[types.ResourceType][]types.Relationship)
	for _, rel := range p.relations {
		groupedRelations[rel.Source.Type] = append(groupedRelations[rel.Source.Type], rel)
	}

	// Order of resource types in visualization
	resourceOrder := []types.ResourceType{
		types.ResourceTypeIngress,
		types.ResourceTypeService,
		types.ResourceTypeDeployment,
		types.ResourceTypePod,
		types.ResourceTypeConfigMap,
	}

	result.WriteString("External Traffic\n│\n")

	for _, resType := range resourceOrder {
		relations, exists := groupedRelations[resType]
		if !exists {
			continue
		}

		result.WriteString("▼\n")
		result.WriteString(fmt.Sprintf("[%s Layer]\n", resType))

		for i, rel := range relations {
			isLast := i == len(relations)-1
			prefix := utils.TreePrefix(isLast)
			result.WriteString(fmt.Sprintf("%s %s\n", prefix, utils.FormatResource(string(rel.Source.Type), rel.Source.Name)))
			result.WriteString(fmt.Sprintf("│   %s %s (%s)\n",
				utils.CreateArrow(4),
				utils.FormatResource(string(rel.Target.Type), rel.Target.Name),
				rel.Description))
		}
	}

	return result.String()
}
