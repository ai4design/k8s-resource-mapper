package mapper

import (
	"context"
	"fmt"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceProcessor handles service resource processing
type ServiceProcessor struct {
	client    *client.K8sClient
	namespace string
	resources []types.Resource
	relations []types.Relationship
}

func NewServiceProcessor(client *client.K8sClient, namespace string) *ServiceProcessor {
	return &ServiceProcessor{
		client:    client,
		namespace: namespace,
	}
}

func (p *ServiceProcessor) Process(ctx context.Context, namespace string) error {
	services, err := p.client.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	for _, svc := range services.Items {
		// Add service resource
		svcResource := types.Resource{
			Type:      types.ResourceTypeService,
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Labels:    svc.Labels,
			Data:      svc,
		}
		p.resources = append(p.resources, svcResource)

		// Find related pods
		if svc.Spec.Selector != nil {
			selector := metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: svc.Spec.Selector,
			})

			pods, err := p.client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: selector,
			})
			if err != nil {
				continue
			}

			// Add relationships for each pod
			for _, pod := range pods.Items {
				podResource := types.Resource{
					Type:      types.ResourceTypePod,
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Labels:    pod.Labels,
					Data:      pod,
				}

				p.relations = append(p.relations, types.Relationship{
					Source:      svcResource,
					Target:      podResource,
					Type:        types.RelationshipTypeTargets,
					Description: fmt.Sprintf("exposes port %d", svc.Spec.Ports[0].Port),
				})
			}
		}

		// Find related ingresses
		ingresses, err := p.client.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, ing := range ingresses.Items {
			for _, rule := range ing.Spec.Rules {
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						if path.Backend.Service.Name == svc.Name {
							ingResource := types.Resource{
								Type:      types.ResourceTypeIngress,
								Name:      ing.Name,
								Namespace: ing.Namespace,
								Labels:    ing.Labels,
								Data:      ing,
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
		}
	}

	return nil
}

func (p *ServiceProcessor) GetRelationships() []types.Relationship {
	return p.relations
}
