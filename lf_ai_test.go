package lifecyclex

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLifecycleAI_Shutdown(t *testing.T) {
	lc := NewLifecycleAI()

	var shutdownOrder []string
	var mu sync.Mutex

	recordShutdown := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		shutdownOrder = append(shutdownOrder, name)
	}

	lc.Add("http-server", func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		recordShutdown("http-server")
		fmt.Println("http-server closed")
		return nil
	})

	lc.Add("user-db", func(ctx context.Context) error {
		time.Sleep(30 * time.Millisecond)
		recordShutdown("user-db")
		fmt.Println("user-db closed")
		return nil
	})

	lc.Add("session-cache", func(ctx context.Context) error {
		time.Sleep(20 * time.Millisecond)
		recordShutdown("session-cache")
		fmt.Println("session-cache closed")
		return nil
	})

	if err := lc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	expectedOrder := []string{"http-server", "session-cache", "user-db"}
	mu.Lock()
	defer mu.Unlock()

	if len(shutdownOrder) != len(expectedOrder) {
		t.Fatalf("Expected shutdown order length %d, got %d. Got order: %v", len(expectedOrder), len(shutdownOrder), shutdownOrder)
	}

	for i, name := range expectedOrder {
		if shutdownOrder[i] != name {
			t.Errorf("Expected shutdownOrder[%d] to be %s, but got %s", i, name, shutdownOrder[i])
		}
	}
}
