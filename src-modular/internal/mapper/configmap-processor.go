package mapper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"k8s-resource-mapper/internal/client"
	"k8s-resource-mapper/internal/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapProcessor handles ConfigMap resource processing
type ConfigMapProcessor struct {
	client     *client.K8sClient
	namespace  string
	resources  []types.Resource
	relations  []types.Relationship
	mu         sync.RWMutex
	visualOpts *types.VisualOptions
}

// NewConfigMapProcessor creates a new ConfigMap processor
func NewConfigMapProcessor(client *client.K8sClient, namespace string, opts *types.VisualOptions) *ConfigMapProcessor {
	return &ConfigMapProcessor{
		client:     client,
		namespace:  namespace,
		visualOpts: opts,
		resources:  make([]types.Resource, 0),
		relations:  make([]types.Relationship, 0),
	}
}

// Process processes ConfigMap resources
func (p *ConfigMapProcessor) Process(ctx context.Context) error {
	configMaps, err := p.client.Clientset.CoreV1().ConfigMaps(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ConfigMaps: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(configMaps.Items))

	for _, cm := range configMaps.Items {
		wg.Add(1)
		go func(c corev1.ConfigMap) {
			defer wg.Done()
			if err := p.processConfigMap(ctx, &c); err != nil {
				errChan <- err
			}
		}(cm)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("ConfigMap processing error: %v", err)
		}
	}

	return nil
}

// processConfigMap processes a single ConfigMap
func (p *ConfigMapProcessor) processConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	// Create ConfigMap resource
	cmResource := types.Resource{
		Type:      types.ResourceTypeConfigMap,
		Name:      cm.Name,
		Namespace: cm.Namespace,
		Labels:    cm.Labels,
		Data:      cm,
		Status:    p.getConfigMapStatus(cm),
		Metrics:   p.getConfigMapMetrics(cm),
	}

	p.addResource(cmResource)

	// Process relationships concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 3) // pods, deployments, statefulsets

	// Process pod usage
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processPodUsage(ctx, cm, cmResource); err != nil {
			errChan <- err
		}
	}()

	// Process deployment usage
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.processDeploymentUsage(ctx, cm, cmResource); err != nil {
			errChan <- err
		}
	}()

	// Process StatefulSet usage (optional)
	if p.visualOpts.ShowExtendedResources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.processStatefulSetUsage(ctx, cm, cmResource); err != nil {
				errChan <- err
			}
		}()
	}

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

// getConfigMapStatus returns the status of a ConfigMap
func (p *ConfigMapProcessor) getConfigMapStatus(cm *corev1.ConfigMap) types.ResourceStatus {
	return types.ResourceStatus{
		Phase:   "Active",
		Ready:   true,
		Details: fmt.Sprintf("Keys: %d", len(cm.Data)+len(cm.BinaryData)),
	}
}

// getConfigMapMetrics returns metrics for a ConfigMap
func (p *ConfigMapProcessor) getConfigMapMetrics(cm *corev1.ConfigMap) types.ResourceMetrics {
	var totalSize int64
	for _, v := range cm.Data {
		totalSize += int64(len(v))
	}
	for _, v := range cm.BinaryData {
		totalSize += int64(len(v))
	}

	return types.ResourceMetrics{
		Keys: len(cm.Data) + len(cm.BinaryData),
		Size: totalSize,
	}
}

// processPodUsage processes pods using the ConfigMap
func (p *ConfigMapProcessor) processPodUsage(ctx context.Context, cm *corev1.ConfigMap, cmResource types.Resource) error {
	pods, err := p.client.Clientset.CoreV1().Pods(cm.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	for _, pod := range pods.Items {
		usageTypes := p.findConfigMapUsageInPod(&pod, cm.Name)
		if len(usageTypes) > 0 {
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
				Source:      podResource,
				Target:      cmResource,
				Type:        types.RelationshipTypeUses,
				Description: fmt.Sprintf("uses as %s", strings.Join(usageTypes, ", ")),
			})
		}
	}

	return nil
}

// processDeploymentUsage processes deployments using the ConfigMap
func (p *ConfigMapProcessor) processDeploymentUsage(ctx context.Context, cm *corev1.ConfigMap, cmResource types.Resource) error {
	deployments, err := p.client.Clientset.AppsV1().Deployments(cm.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %v", err)
	}

	for _, deploy := range deployments.Items {
		usageTypes := p.findConfigMapUsageInPodTemplate(&deploy.Spec.Template, cm.Name)
		if len(usageTypes) > 0 {
			deployResource := types.Resource{
				Type:      types.ResourceTypeDeployment,
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
				Labels:    deploy.Labels,
				Data:      deploy,
				Status: types.ResourceStatus{
					Phase: getDeploymentPhase(&deploy),
					Ready: deploy.Status.ReadyReplicas == *deploy.Spec.Replicas,
				},
			}

			p.addResource(deployResource)
			p.addRelationship(types.Relationship{
				Source:      deployResource,
				Target:      cmResource,
				Type:        types.RelationshipTypeUses,
				Description: fmt.Sprintf("uses as %s", strings.Join(usageTypes, ", ")),
			})
		}
	}

	return nil
}

// processStatefulSetUsage processes StatefulSets using the ConfigMap
func (p *ConfigMapProcessor) processStatefulSetUsage(ctx context.Context, cm *corev1.ConfigMap, cmResource types.Resource) error {
	statefulsets, err := p.client.Clientset.AppsV1().StatefulSets(cm.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list statefulsets: %v", err)
	}

	for _, sts := range statefulsets.Items {
		usageTypes := p.findConfigMapUsageInPodTemplate(&sts.Spec.Template, cm.Name)
		if len(usageTypes) > 0 {
			stsResource := types.Resource{
				Type:      types.ResourceTypeStatefulSet,
				Name:      sts.Name,
				Namespace: sts.Namespace,
				Labels:    sts.Labels,
				Data:      sts,
				Status: types.ResourceStatus{
					Phase: getStatefulSetPhase(&sts),
					Ready: sts.Status.ReadyReplicas == *sts.Spec.Replicas,
				},
			}

			p.addResource(stsResource)
			p.addRelationship(types.Relationship{
				Source:      stsResource,
				Target:      cmResource,
				Type:        types.RelationshipTypeUses,
				Description: fmt.Sprintf("uses as %s", strings.Join(usageTypes, ", ")),
			})
		}
	}

	return nil
}

// Helper functions

func (p *ConfigMapProcessor) addResource(resource types.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, resource)
}

func (p *ConfigMapProcessor) addRelationship(rel types.Relationship) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relations = append(p.relations, rel)
}

func (p *ConfigMapProcessor) findConfigMapUsageInPod(pod *corev1.Pod, configMapName string) []string {
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

	return mapKeysToSlice(usageTypes)
}

func (p *ConfigMapProcessor) findConfigMapUsageInPodTemplate(template *corev1.PodTemplateSpec, configMapName string) []string {
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

	return mapKeysToSlice(usageTypes)
}

func mapKeysToSlice(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GetResources returns the processed resources
func (p *ConfigMapProcessor) GetResources() []types.Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resources
}

// GetRelationships returns the processed relationships
func (p *ConfigMapProcessor) GetRelationships() []types.Relationship {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.relations
}
