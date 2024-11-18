package visualizer

import (
	"fmt"
	"strings"

	"k8s-resource-mapper/internal/types"
	"k8s-resource-mapper/internal/utils"

	corev1 "k8s.io/api/core/v1"
)

// Visualizer handles the visualization of Kubernetes resources and their relationships
type Visualizer struct {
	resourceMapping types.ResourceMapping
	showDetails     bool
	colorOutput     bool
}

// NewVisualizer creates a new Visualizer instance
func NewVisualizer(mapping types.ResourceMapping) *Visualizer {
	return &Visualizer{
		resourceMapping: mapping,
		showDetails:     true,
		colorOutput:     true,
	}
}

// SetOptions configures visualizer options
func (v *Visualizer) SetOptions(showDetails, colorOutput bool) {
	v.showDetails = showDetails
	v.colorOutput = colorOutput
}

const (
	indentSize     = 4
	verticalLine   = "│"
	horizontalLine = "──"
	cornerSymbol   = "└"
	branchSymbol   = "├"
	arrowSymbol    = "➜"
	dotSymbol      = "●"
	warningSymbol  = "⚠"
	successSymbol  = "✓"
	errorSymbol    = "✗"
	detailsSymbol  = "ℹ"
)

// RenderClusterView generates a complete cluster visualization
func (v *Visualizer) RenderClusterView() string {
	var output strings.Builder

	// Header
	output.WriteString(v.renderHeader())
	output.WriteString("\n")

	// Traffic flow
	output.WriteString("External Traffic\n")
	output.WriteString(fmt.Sprintf("%s\n", verticalLine))

	// Ingress Layer
	output.WriteString(v.renderIngressLayer())
	output.WriteString("\n")

	// Service Layer
	output.WriteString(v.renderServiceLayer())
	output.WriteString("\n")

	// Workload Layer
	output.WriteString(v.renderWorkloadLayer())
	output.WriteString("\n")

	// Storage Layer
	output.WriteString(v.renderStorageLayer())

	return output.String()
}

func (v *Visualizer) renderHeader() string {
	return fmt.Sprintf("%s\n%s\n%s",
		utils.ColorizedPrintf(utils.ColorGreen, "Kubernetes Resource Map"),
		strings.Repeat("-", 80),
		utils.ColorizedPrintf(utils.ColorGray, "Generated at: %s", utils.GetCurrentTime()))
}

func (v *Visualizer) renderIngressLayer() string {
	var output strings.Builder
	output.WriteString(utils.ColorizedPrintf(utils.ColorBlue, "[Ingress Layer]\n"))

	ingresses := v.filterResourcesByType(types.ResourceTypeIngress)
	for i, ingress := range ingresses {
		isLast := i == len(ingresses)-1
		prefix := v.getPrefix(isLast)

		// Ingress name and host rules
		output.WriteString(fmt.Sprintf("%s%s %s\n",
			prefix,
			v.colorize(utils.ColorMagenta, dotSymbol),
			v.formatResource(ingress)))

		// Show TLS if configured
		if ing, ok := ingress.Data.(networkingv1.Ingress); ok && len(ing.Spec.TLS) > 0 {
			output.WriteString(fmt.Sprintf("%s  %s TLS Enabled\n",
				v.getIndent(isLast),
				v.colorize(utils.ColorGreen, successSymbol)))
		}

		// Show backend services
		relations := v.findRelationships(ingress, types.RelationshipTypeExposes)
		for j, rel := range relations {
			isLastRel := isLast && j == len(relations)-1
			output.WriteString(fmt.Sprintf("%s  %s %s via %s\n",
				v.getIndent(isLastRel),
				v.colorize(utils.ColorBlue, arrowSymbol),
				v.formatResource(rel.Target),
				rel.Description))
		}
	}

	return output.String()
}

func (v *Visualizer) renderServiceLayer() string {
	var output strings.Builder
	output.WriteString(utils.ColorizedPrintf(utils.ColorBlue, "[Service Layer]\n"))

	services := v.filterResourcesByType(types.ResourceTypeService)
	for i, svc := range services {
		isLast := i == len(services)-1
		prefix := v.getPrefix(isLast)

		// Service name and type
		output.WriteString(fmt.Sprintf("%s%s %s\n",
			prefix,
			v.colorize(utils.ColorBlue, dotSymbol),
			v.formatResource(svc)))

		// Show service details
		if v.showDetails {
			if s, ok := svc.Data.(corev1.Service); ok {
				output.WriteString(fmt.Sprintf("%s  %s Type: %s\n",
					v.getIndent(isLast),
					v.colorize(utils.ColorGray, detailsSymbol),
					s.Spec.Type))
				output.WriteString(fmt.Sprintf("%s  %s Ports: %s\n",
					v.getIndent(isLast),
					v.colorize(utils.ColorGray, detailsSymbol),
					v.formatPorts(s.Spec.Ports)))
			}
		}

		// Show backend pods
		relations := v.findRelationships(svc, types.RelationshipTypeTargets)
		for j, rel := range relations {
			isLastRel := isLast && j == len(relations)-1
			output.WriteString(fmt.Sprintf("%s  %s %s\n",
				v.getIndent(isLastRel),
				v.colorize(utils.ColorGreen, arrowSymbol),
				v.formatResource(rel.Target)))

			// Show pod status
			if pod, ok := rel.Target.Data.(corev1.Pod); ok {
				statusSymbol := v.getPodStatusSymbol(pod.Status.Phase)
				output.WriteString(fmt.Sprintf("%s     %s %s\n",
					v.getIndent(isLastRel),
					statusSymbol,
					pod.Status.Phase))
			}
		}
	}

	return output.String()
}

func (v *Visualizer) renderWorkloadLayer() string {
	var output strings.Builder
	output.WriteString(utils.ColorizedPrintf(utils.ColorBlue, "[Workload Layer]\n"))

	deployments := v.filterResourcesByType(types.ResourceTypeDeployment)
	for i, deploy := range deployments {
		isLast := i == len(deployments)-1
		prefix := v.getPrefix(isLast)

		// Deployment name and status
		output.WriteString(fmt.Sprintf("%s%s %s\n",
			prefix,
			v.colorize(utils.ColorYellow, dotSymbol),
			v.formatResource(deploy)))

		// Show deployment details
		if v.showDetails {
			if d, ok := deploy.Data.(appsv1.Deployment); ok {
				readyReplicas := d.Status.ReadyReplicas
				desiredReplicas := *d.Spec.Replicas
				status := fmt.Sprintf("%d/%d replicas ready", readyReplicas, desiredReplicas)
				statusSymbol := v.getDeploymentStatusSymbol(readyReplicas, desiredReplicas)
				output.WriteString(fmt.Sprintf("%s  %s %s\n",
					v.getIndent(isLast),
					statusSymbol,
					status))
			}
		}

		// Show HPA if configured
		hpaRels := v.findRelationships(deploy, types.RelationshipTypeTargets)
		for _, rel := range hpaRels {
			if rel.Source.Type == types.ResourceTypeHPA {
				output.WriteString(fmt.Sprintf("%s  %s %s\n",
					v.getIndent(isLast),
					v.colorize(utils.ColorCyan, "⟳"),
					rel.Description))
			}
		}

		// Show managed pods
		podRels := v.findRelationships(deploy, types.RelationshipTypeOwns)
		for j, rel := range podRels {
			isLastRel := isLast && j == len(podRels)-1
			output.WriteString(fmt.Sprintf("%s  %s %s\n",
				v.getIndent(isLastRel),
				v.colorize(utils.ColorGreen, arrowSymbol),
				v.formatResource(rel.Target)))
		}
	}

	return output.String()
}

func (v *Visualizer) renderStorageLayer() string {
	var output strings.Builder
	output.WriteString(utils.ColorizedPrintf(utils.ColorBlue, "[Storage Layer]\n"))

	configmaps := v.filterResourcesByType(types.ResourceTypeConfigMap)
	for i, cm := range configmaps {
		isLast := i == len(configmaps)-1
		prefix := v.getPrefix(isLast)

		// ConfigMap name
		output.WriteString(fmt.Sprintf("%s%s %s\n",
			prefix,
			v.colorize(utils.ColorYellow, dotSymbol),
			v.formatResource(cm)))

		// Show usage relationships
		relations := v.findRelationshipsForTarget(cm, types.RelationshipTypeUses)
		for j, rel := range relations {
			isLastRel := isLast && j == len(relations)-1
			output.WriteString(fmt.Sprintf("%s  %s Used by %s (%s)\n",
				v.getIndent(isLastRel),
				v.colorize(utils.ColorBlue, arrowSymbol),
				v.formatResource(rel.Source),
				rel.Description))
		}
	}

	return output.String()
}

// Helper methods...

func (v *Visualizer) formatResource(r types.Resource) string {
	return utils.ColorizedPrintf(utils.GetResourceColor(string(r.Type)), "%s/%s", r.Type, r.Name)
}

func (v *Visualizer) getPrefix(isLast bool) string {
	if isLast {
		return cornerSymbol + horizontalLine + " "
	}
	return branchSymbol + horizontalLine + " "
}

func (v *Visualizer) getIndent(isLast bool) string {
	if isLast {
		return strings.Repeat(" ", indentSize)
	}
	return verticalLine + strings.Repeat(" ", indentSize-1)
}

func (v *Visualizer) colorize(color, text string) string {
	if !v.colorOutput {
		return text
	}
	return utils.Colorize(color, text)
}

func (v *Visualizer) getPodStatusSymbol(phase corev1.PodPhase) string {
	switch phase {
	case corev1.PodRunning:
		return v.colorize(utils.ColorGreen, successSymbol)
	case corev1.PodPending:
		return v.colorize(utils.ColorYellow, warningSymbol)
	default:
		return v.colorize(utils.ColorRed, errorSymbol)
	}
}

func (v *Visualizer) getDeploymentStatusSymbol(ready, desired int32) string {
	if ready == desired {
		return v.colorize(utils.ColorGreen, successSymbol)
	} else if ready > 0 {
		return v.colorize(utils.ColorYellow, warningSymbol)
	}
	return v.colorize(utils.ColorRed, errorSymbol)
}

func (v *Visualizer) formatPorts(ports []corev1.ServicePort) string {
	var portStrings []string
	for _, port := range ports {
		portStr := fmt.Sprintf("%d→%d/%s", port.Port, port.TargetPort.IntVal, port.Protocol)
		portStrings = append(portStrings, portStr)
	}
	return strings.Join(portStrings, ", ")
}

// Helper methods for finding resources and relationships...
func (v *Visualizer) filterResourcesByType(resourceType types.ResourceType) []types.Resource {
	var filtered []types.Resource
	for _, r := range v.resourceMapping.Resources {
		if r.Type == resourceType {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (v *Visualizer) findRelationships(source types.Resource, relType types.RelationshipType) []types.Relationship {
	var filtered []types.Relationship
	for _, r := range v.resourceMapping.Relationships {
		if r.Source.Name == source.Name && r.Source.Type == source.Type && r.Type == relType {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (v *Visualizer) findRelationshipsForTarget(target types.Resource, relType types.RelationshipType) []types.Relationship {
	var filtered []types.Relationship
	for _, r := range v.resourceMapping.Relationships {
		if r.Target.Name == target.Name && r.Target.Type == target.Type && r.Type == relType {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
