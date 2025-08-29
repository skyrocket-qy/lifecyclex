package lifecyclex_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/skyrocket-qy/lifecyclex"
)

func TestSimpleLifecycle(t *testing.T) {
	t.Parallel()

	lc := lifecyclex.NewSimpleLifecycle()

	var order []int

	lc.Add(func(ctx context.Context) error {
		order = append(order, 1)

		return nil
	})
	lc.Add(func(ctx context.Context) error {
		order = append(order, 2)

		return nil
	})
	lc.Add(func(ctx context.Context) error {
		order = append(order, 3)

		return nil
	})

	if err := lc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	expected := []int{3, 2, 1}
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

type app string

type closeEvent struct {
	app      app
	closedAt time.Time
}

func TestLifecycleParallel(t *testing.T) {
	t.Parallel()

	lc := lifecyclex.NewLifecycleParallel()

	var (
		mu         sync.Mutex
		closeOrder []closeEvent
	)

	appA := app("A")
	appB := app("B")
	appC := app("C")
	appD := app("D")
	appE := app("E") // No deps, no one depends on it

	closer := func(a app) lifecyclex.Closer {
		return func(ctx context.Context) error {
			// Simulate work
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()

			closeOrder = append(closeOrder, closeEvent{app: a, closedAt: time.Now()})

			return nil
		}
	}

	// Dependency graph:
	// A -> B -> D
	// A -> C
	// E
	lc.Add(appA, closer(appA), appB, appC)
	lc.Add(appB, closer(appB), appD)
	lc.Add(appC, closer(appC))
	lc.Add(appD, closer(appD))
	lc.Add(appE, closer(appE))

	if err := lc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if len(closeOrder) != 5 {
		t.Fatalf("Expected 5 apps to be closed, but got %d", len(closeOrder))
	}

	// Verification
	closedAt := make(map[app]time.Time)
	for _, event := range closeOrder {
		closedAt[event.app] = event.closedAt
	}

	// B and C must be closed after A
	if closedAt[appB].Before(closedAt[appA]) {
		t.Errorf("App B should be closed after App A, but was closed before")
	}

	if closedAt[appC].Before(closedAt[appA]) {
		t.Errorf("App C should be closed after App A, but was closed before")
	}

	// D must be closed after B
	if closedAt[appD].Before(closedAt[appB]) {
		t.Errorf("App D should be closed after App B, but was closed before")
	}
}
