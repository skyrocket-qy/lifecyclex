# lifecyclex

`lifecyclex` is a Go library that provides utilities for managing application lifecycle, specifically for shutting down components in a structured and safe manner. It offers different strategies for sequential and parallel shutdown of application components based on their dependencies.

## Features

- **Sequential Shutdown:** Shut down components in the reverse order of their initialization.
- **Parallel Shutdown:** Shut down components in parallel, respecting a dependency graph to ensure safe ordering.
- **Error Short-circuiting:** The shutdown process can be configured to stop on the first error encountered.
- **Multiple Implementations:** Choose from different lifecycle managers depending on your needs.

## Installation

To use `lifecyclex` in your project, you can use `go get`:

```bash
go get github.com/skyrocket-qy/lifecyclex
```

## Usage

### SimpleLifecycle

`SimpleLifecycle` provides a straightforward sequential shutdown mechanism. Components are shut down in the reverse order they were added.

```go
package main

import (
	"context"
	"fmt"
	"github.com/skyrocket-qy/lifecyclex"
)

func main() {
	lc := lifecyclex.NewSimpleLifecycle()

	lc.Add(func() error {
		fmt.Println("Closing DB connection")
		return nil
	})

	lc.Add(func() error {
		fmt.Println("Stopping server")
		return nil
	})

	// Shutdown will be called in reverse order: server, then DB.
	if err := lc.Shutdown(context.Background()); err != nil {
		fmt.Printf("Shutdown failed: %v\n", err)
	}
}
```

### LifecycleParallel

`LifecycleParallel` manages shutdown based on a dependency graph, allowing for parallel execution where possible.

```go
package main

import (
	"context"
	"fmt"
	"github.com/skyrocket-qy/lifecyclex"
	"time"
)

func main() {
	lc := lifecyclex.NewLifecycleParallel()

	db := "database"
	server := "server"
	cache := "cache"

	lc.Add(db, func() error {
		fmt.Println("Closing DB connection")
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	lc.Add(server, func() error {
		fmt.Println("Stopping server")
		time.Sleep(50 * time.Millisecond)
		return nil
	}, db, cache) // Server depends on DB and Cache

	lc.Add(cache, func() error {
		fmt.Println("Closing Cache connection")
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// DB and Cache will be closed in parallel, and Server will be closed after them.
	if err := lc.Shutdown(context.Background()); err != nil {
		fmt.Printf("Shutdown failed: %v\n", err)
	}
}
```

## Known Issues and Limitations

This library is currently in an early stage of development, and there are several known limitations and areas for improvement:

- **Context Cancellation:** The `Shutdown` function in `LifecycleParallel` accepts a `context.Context` but does not currently use it to handle cancellations or timeouts.
- **Non-Deterministic Order:** For components at the same level of the dependency graph (i.e., with the same number of dependencies), the shutdown order is not guaranteed.
- **No Cycle Detection:** The library does not detect circular dependencies, which will lead to a deadlock during the shutdown process.

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request.