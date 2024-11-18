package mapper

import (
	"context"
	"fmt"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentProcessor handles deployment resource processing
type DeploymentProcessor struct {
	client     *client.K8sClient
	namespace  string
	resources  []types.Resource
	relations  []types.Relationship
	mu         sync.RWMutex
	visualOpts *types.VisualOptions
}

// NewDeploymentProcessor creates a new deployment processor
func NewDeploymentProcessor(client *client.K8sClient, namespace string, opts *types.VisualOptions) *DeploymentProcessor {
	return &DeploymentProcessor{
		client:     client,
		namespace:  namespace,
		visualOpts: opts,
		resources:  make([]types.Resource, 0),
		relations:  make([]types.Relationship, 0),
	}
}

// Process processes deployment resources
func (p *DeploymentProcessor) Process(ctx context.Context) error {
	deployments, err := p.client.Clientset.AppsV1().Deployments(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(deployments.Items))

	for _, deploy := range deployments.Items {
		wg.Add(1)
		go func(d appsv1.Deployment) {
			defer wg.Done()
			if err := p.processDeployment(ctx, &d); err != nil {
				errChan <- err
			}
		}(deploy)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("deployment processing error: %v", err)
		}
	}

	return nil
}

// processDeployment processes a single deployment
func (p *DeploymentProcessor) processDeployment(ctx context.Context, deploy *appsv1.Deployment) error {
	// Create deployment resource
	deployResource := types.Resource{
		Type:      types.ResourceTypeDeployment,
		Name:      deploy.Name,
		Namespace: deploy.Namespace,
		Labels:    deploy.Labels,
		Data:      deploy,
		Status:    p.getDeploymentStatus(deploy),
		Metrics:   p.getDeploymentMetrics(deploy),
	}

	p.addResource(deployResource)

	// Process related resources concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 3) // pods, hpa, configmaps

	// Process pods
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processPods(ctx, deploy, deployResource); err != nil {
			errChan <- err
		}
	}()

	// Process HPA
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processHPA(ctx, deploy, deployResource); err != nil {
			errChan <- err
		}
	}()

	// Process ConfigMaps
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processConfigMaps(ctx, deploy, deployResource); err != nil {
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

// getDeploymentStatus returns the status of a deployment
func (p *DeploymentProcessor) getDeploymentStatus(deploy *appsv1.Deployment) types.ResourceStatus {
	status := types.ResourceStatus{
		Phase: "Unknown",
		Ready: false,
	}

	if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
		status.Phase = "Ready"
		status.Ready = true
	} else if deploy.Status.ReadyReplicas > 0 {
		status.Phase = "PartiallyReady"
		status.Ready = false
	} else {
		status.Phase = "NotReady"
		status.Ready = false
	}

	status.Details = fmt.Sprintf("%d/%d replicas ready",
		deploy.Status.ReadyReplicas, *deploy.Spec.Replicas)

	return status
}

// getDeploymentMetrics returns metrics for a deployment
func (p *DeploymentProcessor) getDeploymentMetrics(deploy *appsv1.Deployment) types.ResourceMetrics {
	metrics := types.ResourceMetrics{
		CPU:    "N/A",
		Memory: "N/A",
		Pods:   int(*deploy.Spec.Replicas),
	}

	if deploy.Spec.Template.Spec.Containers != nil && len(deploy.Spec.Template.Spec.Containers) > 0 {
		container := deploy.Spec.Template.Spec.Containers[0]
		if container.Resources.Requests != nil {
			metrics.CPU = container.Resources.Requests.Cpu().String()
			metrics.Memory = container.Resources.Requests.Memory().String()
		}
	}

	return metrics
}

// processPods processes pods related to a deployment
func (p *DeploymentProcessor) processPods(ctx context.Context, deploy *appsv1.Deployment, deployResource types.Resource) error {
	selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		return fmt.Errorf("invalid selector: %v", err)
	}

	pods, err := p.client.Clientset.CoreV1().Pods(deploy.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
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
			Source:      deployResource,
			Target:      podResource,
			Type:        types.RelationshipTypeOwns,
			Description: "manages pod",
		})
	}

	return nil
}

// processHPA processes HPA related to a deployment
func (p *DeploymentProcessor) processHPA(ctx context.Context, deploy *appsv1.Deployment, deployResource types.Resource) error {
	hpas, err := p.client.Clientset.AutoscalingV2().HorizontalPodAutoscalers(deploy.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list HPAs: %v", err)
	}

	for _, hpa := range hpas.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == deploy.Name {
			hpaResource := types.Resource{
				Type:      types.ResourceTypeHPA,
				Name:      hpa.Name,
				Namespace: hpa.Namespace,
				Labels:    hpa.Labels,
				Data:      hpa,
				Status: types.ResourceStatus{
					Phase:   "Active",
					Ready:   true,
					Details: fmt.Sprintf("scales %d-%d replicas", *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas),
				},
			}

			p.addResource(hpaResource)
			p.addRelationship(types.Relationship{
				Source:      hpaResource,
				Target:      deployResource,
				Type:        types.RelationshipTypeTargets,
				Description: fmt.Sprintf("scales %d-%d replicas", *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas),
			})
		}
	}

	return nil
}

// processConfigMaps processes ConfigMaps used by a deployment
func (p *DeploymentProcessor) processConfigMaps(ctx context.Context, deploy *appsv1.Deployment, deployResource types.Resource) error {
	configMaps := make(map[string]bool)

	// Check volumes
	for _, volume := range deploy.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			configMaps[volume.ConfigMap.Name] = true
		}
	}

	// Check containers
	for _, container := range deploy.Spec.Template.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				configMaps[envFrom.ConfigMapRef.Name] = true
			}
		}

		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				configMaps[env.ValueFrom.ConfigMapKeyRef.Name] = true
			}
		}
	}

	// Process each ConfigMap
	for cmName := range configMaps {
		cm, err := p.client.Clientset.CoreV1().ConfigMaps(deploy.Namespace).Get(ctx, cmName, metav1.GetOptions{})
		if err != nil {
			continue // Skip if ConfigMap not found
		}

		cmResource := types.Resource{
			Type:      types.ResourceTypeConfigMap,
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Labels:    cm.Labels,
			Data:      cm,
			Status: types.ResourceStatus{
				Phase: "Active",
				Ready: true,
			},
		}

		p.addResource(cmResource)
		p.addRelationship(types.Relationship{
			Source:      deployResource,
			Target:      cmResource,
			Type:        types.RelationshipTypeUses,
			Description: "uses config",
		})
	}

	return nil
}

// Helper functions...

func (p *DeploymentProcessor) addResource(resource types.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, resource)
}

func (p *DeploymentProcessor) addRelationship(rel types.Relationship) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relations = append(p.relations, rel)
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// GetResources returns the processed resources
func (p *DeploymentProcessor) GetResources() []types.Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resources
}

// GetRelationships returns the processed relationships
func (p *DeploymentProcessor) GetRelationships() []types.Relationship {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.relations
}
