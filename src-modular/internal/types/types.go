package types

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// ResourceType represents the type of Kubernetes resource
type ResourceType string

// Resource types
const (
	ResourceTypeNamespace  ResourceType = "Namespace"
	ResourceTypePod        ResourceType = "Pod"
	ResourceTypeService    ResourceType = "Service"
	ResourceTypeIngress    ResourceType = "Ingress"
	ResourceTypeConfigMap  ResourceType = "ConfigMap"
	ResourceTypeDeployment ResourceType = "Deployment"
	ResourceTypeHPA        ResourceType = "HPA"
	ResourceTypeSecret     ResourceType = "Secret"
)

// Resource represents a generic Kubernetes resource
type Resource struct {
	Type      ResourceType
	Name      string
	Namespace string
	Labels    map[string]string
	Data      interface{}
}

// RelationshipType represents the type of relationship between resources
type RelationshipType string

// Relationship types
const (
	RelationshipTypeOwns     RelationshipType = "owns"
	RelationshipTypeUses     RelationshipType = "uses"
	RelationshipTypeExposes  RelationshipType = "exposes"
	RelationshipTypeTargets  RelationshipType = "targets"
	RelationshipTypeProvides RelationshipType = "provides"
)

// Relationship represents a relationship between two resources
type Relationship struct {
	Source      Resource
	Target      Resource
	Type        RelationshipType
	Description string
}

// ResourceProcessor interface for processing different resource types
type ResourceProcessor interface {
	Process(ctx context.Context, namespace string) error
	GetRelationships() []Relationship
}

// ResourceMapping holds the mapping of resources and their relationships
type ResourceMapping struct {
	Resources     []Resource
	Relationships []Relationship
}

// DeploymentInfo contains processed deployment information
type DeploymentInfo struct {
	Deployment *appsv1.Deployment
	Pods       []corev1.Pod
	Services   []corev1.Service
	ConfigMaps []corev1.ConfigMap
	HPA        *autoscalingv2.HorizontalPodAutoscaler
}

// ServiceInfo contains processed service information
type ServiceInfo struct {
	Service  *corev1.Service
	Pods     []corev1.Pod
	Ingress  []networkingv1.Ingress
	Endpoint *corev1.Endpoints
}

// ConfigMapInfo contains processed configmap information
type ConfigMapInfo struct {
	ConfigMap *corev1.ConfigMap
	UsedBy    []Resource
}

// ResourceMetrics contains resource usage metrics
type ResourceMetrics struct {
	CPU    string
	Memory string
	Pods   int
}
