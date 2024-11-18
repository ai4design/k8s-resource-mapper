```go
package common

import (
	"fmt"
	"strings"
	"sync"

	"k8s-resource-mapper/internal/types"
)

// ResourceKey generates a unique key for a resource
func ResourceKey(resource types.Resource) string {
	return fmt.Sprintf("%s/%s/%s", resource.Type, resource.Namespace, resource.Name)
}

// ResourceRefKey generates a unique key for a resource reference
func ResourceRefKey(ref ResourceRef) string {
	return fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name)
}

// SafeStringMap provides a thread-safe string map
type SafeStringMap struct {
	sync.RWMutex
	data map[string]string
}

// NewSafeStringMap creates a new thread-safe string map
func NewSafeStringMap() *SafeStringMap {
	return &SafeStringMap{
		data: make(map[string]string),
	}
}

func (m *SafeStringMap) Set(key, value string) {
	m.Lock()
	defer m.Unlock()
	m.data[key] = value
}

func (m *SafeStringMap) Get(key string) (string, bool) {
	m.RLock()
	defer m.RUnlock()
	value, exists := m.data[key]
	return value, exists
}

// ResourceSet provides a thread-safe set of resources
type ResourceSet struct {
	sync.RWMutex
	data map[string]types.Resource
}

// NewResourceSet creates a new resource set
func NewResourceSet() *ResourceSet {
	return &ResourceSet{
		data: make(map[string]types.Resource),
	}
}

func (s *ResourceSet) Add(resource types.Resource) {
	s.Lock()
	defer s.Unlock()
	s.data[ResourceKey(resource)] = resource
}

func (s *ResourceSet) Contains(resource types.Resource) bool {
	s.RLock()
	defer s.RUnlock()
	_, exists := s.data[ResourceKey(resource)]
	return exists
}

func (s *ResourceSet) ToSlice() []types.Resource {
	s.RLock()
	defer s.RUnlock()
	result := make([]types.Resource, 0, len(s.data))
	for _, resource := range s.data {
		result = append(result, resource)
	}
	return result
}
```
