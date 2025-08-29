package lifecyclex

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type LifecycleAI struct {
	components map[string]Closer
}

func NewLifecycleAI() *LifecycleAI {
	return &LifecycleAI{
		components: make(map[string]Closer),
	}
}

func (l *LifecycleAI) Add(name string, closer Closer) {
	l.components[name] = closer
}

// Shutdown executes the shutdown process by inferring component dependencies based on their names.
func (l *LifecycleAI) Shutdown(ctx context.Context) error {
	// Define keyword priorities for dependency inference.
	priorities := map[string]int{
		"server":   100,
		"api":      100,
		"http":     100,
		"grpc":     100,
		"cache":    50,
		"redis":    50,
		"memcached": 50,
		"db":       10,
		"database": 10,
		"postgres": 10,
		"mysql":    10,
	}

	// Assign priorities to components and group them.
	grouped := make(map[int][]*namedCloser)
	for name, closer := range l.components {
		priority := 0 // Default priority
		for keyword, p := range priorities {
			if strings.Contains(strings.ToLower(name), keyword) {
				priority = p
				break
			}
		}
		grouped[priority] = append(grouped[priority], &namedCloser{name, closer})
	}

	// Get sorted list of priorities.
	var sortedPriorities []int
	for p := range grouped {
		sortedPriorities = append(sortedPriorities, p)
	}
	// Sort in descending order.
	sort.Slice(sortedPriorities, func(i, j int) bool {
		return sortedPriorities[i] > sortedPriorities[j]
	})

	// Shutdown components in order of priority.
	for _, p := range sortedPriorities {
		var wg sync.WaitGroup
		errCh := make(chan error, len(grouped[p]))

		for _, nc := range grouped[p] {
			wg.Add(1)
			go func(nc *namedCloser) {
				defer wg.Done()
				if err := nc.closer(ctx); err != nil {
					errCh <- fmt.Errorf("failed to close %s: %w", nc.name, err)
				}
			}(nc)
		}

		wg.Wait()
		close(errCh)

		var errs []string
		for err := range errCh {
			errs = append(errs, err.Error())
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
	}

	return nil
}

type namedCloser struct {
	name   string
	closer Closer
}
