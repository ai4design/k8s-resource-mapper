package common

import (
	"regexp"
	"strings"

	"k8s-resource-mapper/internal/types"
)

// ResourceFilter defines a filter for resources
type ResourceFilter struct {
	Types      []string
	Namespaces []string
	LabelMatch map[string]string
	NameMatch  string
}

// RelationshipFilter defines a filter for relationships
type RelationshipFilter struct {
	Types          []string
	SourceTypes    []string
	TargetTypes    []string
	NamespaceScope bool
}

// FilterResources filters resources based on the provided criteria
func FilterResources(resources []types.Resource, filter ResourceFilter) []types.Resource {
	if len(resources) == 0 {
		return resources
	}

	var result []types.Resource
	nameRegex := regexp.MustCompile(filter.NameMatch)

	for _, resource := range resources {
		// Check resource type
		if len(filter.Types) > 0 && !contains(filter.Types, string(resource.Type)) {
			continue
		}

		// Check namespace
		if len(filter.Namespaces) > 0 && !contains(filter.Namespaces, resource.Namespace) {
			continue
		}

		// Check name pattern
		if filter.NameMatch != "" && !nameRegex.MatchString(resource.Name) {
			continue
		}

		// Check labels
		if !matchLabels(resource.Labels, filter.LabelMatch) {
			continue
		}

		result = append(result, resource)
	}

	return result
}

// FilterRelationships filters relationships based on the provided criteria
func FilterRelationships(relations []types.Relationship, filter RelationshipFilter) []types.Relationship {
	if len(relations) == 0 {
		return relations
	}

	var result []types.Relationship
	for _, rel := range relations {
		// Check relationship type
		if len(filter.Types) > 0 && !contains(filter.Types, string(rel.Type)) {
			continue
		}

		// Check source type
		if len(filter.SourceTypes) > 0 && !contains(filter.SourceTypes, string(rel.Source.Type)) {
			continue
		}

		// Check target type
		if len(filter.TargetTypes) > 0 && !contains(filter.TargetTypes, string(rel.Target.Type)) {
			continue
		}

		// Check namespace scope
		if filter.NamespaceScope && rel.Source.Namespace != rel.Target.Namespace {
			continue
		}

		result = append(result, rel)
	}

	return result
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func matchLabels(resourceLabels, filterLabels map[string]string) bool {
	if len(filterLabels) == 0 {
		return true
	}

	for k, v := range filterLabels {
		resourceValue, exists := resourceLabels[k]
		if !exists || !matchLabelValue(resourceValue, v) {
			return false
		}
	}

	return true
}

func matchLabelValue(value, pattern string) bool {
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(value, strings.Trim(pattern, "*"))
	} else if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(value, strings.TrimPrefix(pattern, "*"))
	} else if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return value == pattern
}
