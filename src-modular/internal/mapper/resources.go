package mapper

import (
	"context"
	"fmt"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentProcessor handles deployment resource processing
type DeploymentProcessor struct {
	client    *client.K8sClient
	namespace string
	resources []types.Resource
	relations []types.Relationship
}

func NewDeploymentProcessor(client *client.K8sClient, namespace string) *DeploymentProcessor {
	return &DeploymentProcessor{
		client:    client,
		namespace: namespace,
	}
}

func (p *DeploymentProcessor) Process(ctx context.Context, namespace string) error {
	deployments, err := p.client.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %v", err)
	}

	for _, deploy := range deployments.Items {
		// Add deployment resource
		deployResource := types.Resource{
			Type:      types.ResourceTypeDeployment,
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
			Labels:    deploy.Labels,
			Data:      deploy,
		}
		p.resources = append(p.resources, deployResource)

		// Find related pods
		selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
		if err != nil {
			continue
		}

		pods, err := p.client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
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
			p.resources = append(p.resources, podResource)

			p.relations = append(p.relations, types.Relationship{
				Source:      deployResource,
				Target:      podResource,
				Type:        types.RelationshipTypeOwns,
				Description: "controls pod",
			})
		}

		// Check for HPA
		hpas, err := p.client.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, hpa := range hpas.Items {
			if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == deploy.Name {
				hpaResource := types.Resource{
					Type:      types.ResourceTypeHPA,
					Name:      hpa.Name,
					Namespace: hpa.Namespace,
					Labels:    hpa.Labels,
					Data:      hpa,
				}
				p.resources = append(p.resources, hpaResource)

				p.relations = append(p.relations, types.Relationship{
					Source: hpaResource,
					Target: deployResource,
					Type:   types.RelationshipTypeTargets,
					Description: fmt.Sprintf("scales between %d and %d replicas",
						*hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas),
				})
			}
		}
	}

	return nil
}

func (p *DeploymentProcessor) GetRelationships() []types.Relationship {
	return p.relations
}

// ResourceInfo holds processed information about a resource
type ResourceInfo struct {
	Resource     types.Resource
	Metrics      types.ResourceMetrics
	Dependencies []types.Relationship
}

// GetResourceInfo returns detailed information about a specific resource
func GetResourceInfo(ctx context.Context, client *client.K8sClient,
	resourceType types.ResourceType, name, namespace string) (*ResourceInfo, error) {

	var resource types.Resource
	var metrics types.ResourceMetrics
	var dependencies []types.Relationship

	switch resourceType {
	case types.ResourceTypeDeployment:
		deploy, err := client.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resource = types.Resource{
			Type:      types.ResourceTypeDeployment,
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
			Labels:    deploy.Labels,
			Data:      deploy,
		}

		// Get metrics
		metrics = types.ResourceMetrics{
			Pods:   int(*deploy.Spec.Replicas),
			CPU:    deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String(),
			Memory: deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String(),
		}

	case types.ResourceTypePod:
		pod, err := client.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		resource = types.Resource{
			Type:      types.ResourceTypePod,
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
			Data:      pod,
		}

		// Get metrics
		if pod.Spec.Containers != nil && len(pod.Spec.Containers) > 0 {
			metrics = types.ResourceMetrics{
				CPU:    pod.Spec.Containers[0].Resources.Requests.Cpu().String(),
				Memory: pod.Spec.Containers[0].Resources.Requests.Memory().String(),
			}
		}
	}

	return &ResourceInfo{
		Resource:     resource,
		Metrics:      metrics,
		Dependencies: dependencies,
	}, nil
}
