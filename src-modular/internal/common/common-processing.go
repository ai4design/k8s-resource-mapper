```go
package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s-resource-mapper/internal/types"
)

// ProcessingResult holds the result of a processing operation
type ProcessingResult struct {
	Resources []types.Resource
	Relations []types.Relationship
	Metrics   ProcessorMetrics
	Errors    []error
}

// ProcessingQueue manages concurrent resource processing
type ProcessingQueue struct {
	wg       sync.WaitGroup
	errChan  chan error
	ctx      context.Context
	cancel   context.CancelFunc
	metrics  *ProcessorMetrics
	metricMu sync.Mutex
}

// NewProcessingQueue creates a new processing queue
func NewProcessingQueue(parentCtx context.Context) *ProcessingQueue {
	ctx, cancel := context.WithCancel(parentCtx)
	return &ProcessingQueue{
		errChan: make(chan error, 100),
		ctx:     ctx,
		cancel:  cancel,
		metrics: &ProcessorMetrics{},
	}
}

// AddTask adds a processing task to the queue
func (q *ProcessingQueue) AddTask(task func() error) {
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		if err := task(); err != nil {
			select {
			case q.errChan <- err:
			default:
				// Channel full, increment error count
				q.metricMu.Lock()
				q.metrics.ErrorCount++
				q.metricMu.Unlock()
			}
		}
		q.metricMu.Lock()
		q.metrics.ResourcesProcessed++
		q.metricMu.Unlock()
	}()
}

// Wait waits for all tasks to complete and returns any errors
func (q *ProcessingQueue) Wait() []error {
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	var errors []error
	errorCollector := make(chan error)

	go func() {
		for err := range q.errChan {
			errorCollector <- err
		}
		close(errorCollector)
	}()

	select {
	case <-done:
		close(q.errChan)
		for err := range errorCollector {
			errors = append(errors, err)
		}
	case <-q.ctx.Done():
		errors = append(errors, fmt.Errorf("processing cancelled: %v", q.ctx.Err()))
	}

	return errors
}

// ResourceProcessor defines an interface for resource processors
type ResourceProcessor interface {
	Process(context.Context) error
	GetResources() []types.Resource
	GetRelationships() []types.Relationship
}

// ProcessResources processes resources using multiple processors concurrently
func ProcessResources(ctx context.Context, processors []ResourceProcessor, opts ProcessorOptions) (*ProcessingResult, error) {
	startTime := time.Now()
	result := &ProcessingResult{
		Resources: make([]types.Resource, 0),
		Relations: make([]types.Relationship, 0),
		Metrics: ProcessorMetrics{
			ProcessingTime: 0,
			ErrorCount:    0,
		},
	}

	queue := NewProcessingQueue(ctx)
	resourceSet := NewResourceSet()

	// Process resources concurrently
	for _, processor := range processors {
		proc := processor // Capture processor variable
		queue.AddTask(func() error {
			if err := proc.Process(ctx); err != nil {
				return fmt.Errorf("processor error: %v", err)
			}

			// Collect resources and relationships
			for _, resource := range proc.GetResources() {
				if !resourceSet.Contains(resource) {
					resourceSet.Add(resource)
					result.Resources = append(result.Resources, resource)
				}
			}

			relations := proc.GetRelationships()
			result.Relations = append(result.Relations, relations...)
			result.Metrics.RelationsDiscovered += len(relations)

			return nil
		})
	}

	// Wait for all processors to complete
	if errs := queue.Wait(); len(errs) > 0 {
		result.Errors = errs
		result.Metrics.ErrorCount = len(errs)
	}

	result.Metrics.ProcessingTime = time.Since(startTime)
	result.Metrics.ResourcesProcessed = len(result.Resources)

	return result, nil
}
```
