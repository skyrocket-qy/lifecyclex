package lifecyclex_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/skyrocket-qy/erx"
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

func TestSimpleLifecycle_Error(t *testing.T) {
	t.Parallel()

	lc := lifecyclex.NewSimpleLifecycle()

	var order []int
	testErr := errors.New("test error")

	lc.Add(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	lc.Add(func(ctx context.Context) error {
		order = append(order, 2)
		return testErr
	})
	lc.Add(func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	err := lc.Shutdown(context.Background())
	if !errors.Is(err, testErr) {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	expected := []int{3, 2}
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
	appE := app("E")

	closer := func(a app) lifecyclex.Closer {
		return func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			closeOrder = append(closeOrder, closeEvent{app: a, closedAt: time.Now()})
			return nil
		}
	}

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

	closedAt := make(map[app]time.Time)
	for _, event := range closeOrder {
		closedAt[event.app] = event.closedAt
	}

	if closedAt[appB].Before(closedAt[appA]) {
		t.Errorf("App B should be closed after App A")
	}
	if closedAt[appC].Before(closedAt[appA]) {
		t.Errorf("App C should be closed after App A")
	}
	if closedAt[appD].Before(closedAt[appB]) {
		t.Errorf("App D should be closed after App B")
	}
}

func TestLifecycleParallel_DanglingDependency(t *testing.T) {
	t.Parallel()

	lc := lifecyclex.NewLifecycleParallel()

	appA := app("A")
	appB := app("B")

	lc.Add(appA, func(ctx context.Context) error {
		return nil
	}, appB)

	if err := lc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestLifecycleParallel_Error(t *testing.T) {
	t.Parallel()

	lc := lifecyclex.NewLifecycleParallel()

	var (
		closed     []app
		closedLock sync.Mutex
	)

	appA := app("A")
	appB := app("B")
	appC := app("C")
	appD := app("D")

	testErr := errors.New("test error")

	closer := func(a app, err error, delay time.Duration) lifecyclex.Closer {
		return func(ctx context.Context) error {
			// Check context before sleeping to ensure short-circuiting works
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(delay)
			closedLock.Lock()
			defer closedLock.Unlock()
			closed = append(closed, a)
			return err
		}
	}

	// Chain 1: A -> B. B will fail.
	// Chain 2: C -> D. C will be slow, D should not be closed.
	lc.Add(appA, closer(appA, nil, 0), appB)
	lc.Add(appB, closer(appB, testErr, 10*time.Millisecond))
	lc.Add(appC, closer(appC, nil, 20*time.Millisecond), appD)
	lc.Add(appD, closer(appD, nil, 0))

	err := lc.Shutdown(context.Background())
	if !errors.Is(err, testErr) {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	closedLock.Lock()
	defer closedLock.Unlock()

	closedMap := make(map[app]bool)
	for _, a := range closed {
		closedMap[a] = true
	}

	if !closedMap[appA] {
		t.Errorf("App A should have been closed")
	}
	if !closedMap[appB] {
		t.Errorf("App B should have been closed (attempted)")
	}
	// C is slow, by the time it finishes, context should be cancelled.
	// So D should not be closed.
	if closedMap[appD] {
		t.Errorf("App D should not have been closed")
	}
}

func TestLifecycleParallel_Unreachable(t *testing.T) {
	t.Parallel()
	lc := lifecyclex.NewLifecycleParallel()
	lc.Add("A", func(ctx context.Context) error {
		return erx.Newf(erx.ErrUnknown, "failed to shutdown: %v", "some string")
	})
	if err := lc.Shutdown(context.Background()); err == nil {
		t.Error("expected an error")
	}
}
