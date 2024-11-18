package mapper

import (
	"context"
	"fmt"
	"strings"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapProcessor handles ConfigMap resource processing
type ConfigMapProcessor struct {
	client    *client.K8sClient
	namespace string
	resources []types.Resource
	relations []types.Relationship
}

func NewConfigMapProcessor(client *client.K8sClient, namespace string) *ConfigMapProcessor {
	return &ConfigMapProcessor{
		client:    client,
		namespace: namespace,
	}
}

func (p *ConfigMapProcessor) Process(ctx context.Context, namespace string) error {
	configmaps, err := p.client.Clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list configmaps: %v", err)
	}

	for _, cm := range configmaps.Items {
		cmResource := types.Resource{
			Type:      types.ResourceTypeConfigMap,
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Labels:    cm.Labels,
			Data:      cm,
		}
		p.resources = append(p.resources, cmResource)

		// Find pods using this ConfigMap
		pods, err := p.client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			if uses, usageTypes := p.findConfigMapUsage(&pod, cm.Name); uses {
				podResource := types.Resource{
					Type:      types.ResourceTypePod,
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Labels:    pod.Labels,
					Data:      pod,
				}

				p.relations = append(p.relations, types.Relationship{
					Source:      podResource,
					Target:      cmResource,
					Type:        types.RelationshipTypeUses,
					Description: fmt.Sprintf("uses as %s", usageTypes),
				})
			}
		}

		// Check deployments for ConfigMap usage
		deployments, err := p.client.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, deploy := range deployments.Items {
			if uses, usageTypes := p.findConfigMapUsageInPodTemplate(&deploy.Spec.Template, cm.Name); uses {
				deployResource := types.Resource{
					Type:      types.ResourceTypeDeployment,
					Name:      deploy.Name,
					Namespace: deploy.Namespace,
					Labels:    deploy.Labels,
					Data:      deploy,
				}

				p.relations = append(p.relations, types.Relationship{
					Source:      deployResource,
					Target:      cmResource,
					Type:        types.RelationshipTypeUses,
					Description: fmt.Sprintf("uses as %s", usageTypes),
				})
			}
		}
	}

	return nil
}

func (p *ConfigMapProcessor) GetRelationships() []types.Relationship {
	return p.relations
}

func (p *ConfigMapProcessor) findConfigMapUsage(pod *corev1.Pod, configMapName string) (bool, string) {
	usageTypes := make(map[string]bool)

	// Check volumes
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name == configMapName {
			usageTypes["volume"] = true
		}
	}

	// Check containers
	for _, container := range pod.Spec.Containers {
		// Check envFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
				usageTypes["environment"] = true
			}
		}

		// Check env
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
				env.ValueFrom.ConfigMapKeyRef.Name == configMapName {
				usageTypes["environment variable"] = true
			}
		}
	}

	if len(usageTypes) == 0 {
		return false, ""
	}

	// Convert usage types to string
	var usageList []string
	for usage := range usageTypes {
		usageList = append(usageList, usage)
	}

	return true, strings.Join(usageList, ", ")
}

func (p *ConfigMapProcessor) findConfigMapUsageInPodTemplate(template *corev1.PodTemplateSpec,
	configMapName string) (bool, string) {
	usageTypes := make(map[string]bool)

	// Check volumes
	for _, volume := range template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name == configMapName {
			usageTypes["volume"] = true
		}
	}

	// Check containers
	for _, container := range template.Spec.Containers {
		// Check envFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
				usageTypes["environment"] = true
			}
		}

		// Check env
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
				env.ValueFrom.ConfigMapKeyRef.Name == configMapName {
				usageTypes["environment variable"] = true
			}
		}
	}

	if len(usageTypes) == 0 {
		return false, ""
	}

	// Convert usage types to string
	var usageList []string
	for usage := range usageTypes {
		usageList = append(usageList, usage)
	}

	return true, strings.Join(usageList, ", ")
}
