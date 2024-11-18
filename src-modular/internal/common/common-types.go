```go
package common

import (
	"time"

	"k8s-resource-mapper/internal/types"
)

// ResourceCollection holds all discovered resources and relationships
type ResourceCollection struct {
	Resources     []types.Resource
	Relationships []types.Relationship
	Timestamp     time.Time
}

// ProcessorOptions holds configuration for resource processors
type ProcessorOptions struct {
	ShowDetails          bool
	IncludeMetrics      bool
	MaxDepth            int
	FollowDependencies  bool
	ExcludedTypes       []string
	NamespaceRestricted bool
}

// ProcessorMetrics holds metrics about the processing
type ProcessorMetrics struct {
	ResourcesProcessed  int
	RelationsDiscovered int
	ProcessingTime     time.Duration
	ErrorCount         int
}

// ResourceRef is a lightweight reference to a resource
type ResourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

// RelationshipRef holds a relationship between two resource references
type RelationshipRef struct {
	Source      ResourceRef
	Target      ResourceRef
	Type        string
	Description string
}
```
