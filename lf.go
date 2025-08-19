package main

import (
	"context"
	"sync"
	"sync/atomic"
)

type Closer func() error

type SimpleLifecycle struct {
	closers []Closer
}

func NewSimpleLifecycle() *SimpleLifecycle {
	return &SimpleLifecycle{}
}

func (l *SimpleLifecycle) Add(fn Closer) {
	l.closers = append(l.closers, fn)
}

func (l *SimpleLifecycle) Shutdown(ctx context.Context) error {
	for i := len(l.closers) - 1; i >= 0; i-- {
		if err := l.closers[i](); err != nil {
			return err
		}
	}

	return nil
}

type LifecycleParallel struct {
	appUpstreams map[any][]any
	appCloser    map[any]Closer

	appIndegrees      map[any]int
	appIndegreesMutex *sync.Mutex
	readyCh           chan any
}

func NewLifecycleParallel() *LifecycleParallel {
	return &LifecycleParallel{
		appUpstreams: make(map[any][]any),
		appCloser:    make(map[any]Closer),
	}
}

func (l *LifecycleParallel) Add(app any, fn Closer, deps ...any) {
	l.appUpstreams[app] = deps
	l.appCloser[app] = fn
}

func (l *LifecycleParallel) Finish() {
	l.appIndegrees = map[any]int{}
	for app, ups := range l.appUpstreams {
		if _, ok := l.appIndegrees[app]; !ok {
			l.appIndegrees[app] = 0
		}

		for _, up := range ups {
			if _, ok := l.appCloser[up]; !ok {
				continue
			}
			l.appIndegrees[up]++
		}
	}

	l.appIndegreesMutex = &sync.Mutex{}
	l.readyCh = make(chan any, len(l.appIndegrees))
	for app, indegree := range l.appIndegrees {
		if indegree == 0 {
			l.readyCh <- app
		}
	}
}

func (l *LifecycleParallel) Shutdown(c context.Context) error {
	l.Finish()
	var wg sync.WaitGroup
	var firstErr atomic.Value
	wg.Add(len(l.readyCh))

	go func() {
		wg.Wait()
		close(l.readyCh)
	}()

	for app := range l.readyCh {
		go func(app any) {
			defer wg.Done()

			if firstErr.Load() != nil {
				return
			}

			if closer, ok := l.appCloser[app]; ok {
				if err := closer(); err != nil {
					firstErr.Store(err)
					return
				}
			}

			for _, up := range l.appUpstreams[app] {
				l.appIndegreesMutex.Lock()
				l.appIndegrees[up]--
				v := l.appIndegrees[up]
				l.appIndegreesMutex.Unlock()
				if v == 0 {
					wg.Add(1)
					l.readyCh <- up
				}
			}
		}(app)
	}

	if err := firstErr.Load(); err != nil {
		return err.(error)
	}

	return nil
}

type LifecycleParallelSyncMap struct {
	appUpstreams map[any][]any
	appCloser    map[any]Closer

	appIndegrees sync.Map
	it           int
	readyCh      chan any
}

func NewLifecycleParallelSyncMap() *LifecycleParallelSyncMap {
	return &LifecycleParallelSyncMap{
		appUpstreams: make(map[any][]any),
		appCloser:    make(map[any]Closer),
	}
}

func (l *LifecycleParallelSyncMap) Add(app any, fn Closer, deps ...any) {
	l.appUpstreams[app] = deps
	l.appCloser[app] = fn
}

func (l *LifecycleParallelSyncMap) Finish() {
	l.appIndegrees = sync.Map{}

	for app, ups := range l.appUpstreams {
		if _, ok := l.appIndegrees.Load(app); !ok {
			l.appIndegrees.Store(app, new(int32))
		}

		for _, up := range ups {
			actual, loaded := l.appIndegrees.LoadOrStore(up, new(int32))
			val := actual.(*int32)
			if loaded {
				atomic.AddInt32(val, 1) // already existed, increment
			} else {
				atomic.StoreInt32(val, 1) // just created, set to 1
			}
		}
	}

	l.readyCh = make(chan any, len(l.appUpstreams))
	l.appIndegrees.Range(func(key, value any) bool {
		if atomic.LoadInt32(value.(*int32)) == 0 {
			l.readyCh <- key
			l.it++
		}
		return true
	})
}

func (l *LifecycleParallelSyncMap) Shutdown() error {
	var wg sync.WaitGroup
	var firstErr atomic.Value

	wg.Add(l.it)

	go func() {
		wg.Wait()
		close(l.readyCh)
	}()

	for app := range l.readyCh {
		go func(app any) {
			defer wg.Done()

			if firstErr.Load() != nil {
				return
			}

			if closer, ok := l.appCloser[app]; ok {
				if err := closer(); err != nil {
					firstErr.Store(err)
					return
				}
			}

			for _, up := range l.appUpstreams[app] {
				if val, ok := l.appIndegrees.Load(up); ok {
					if atomic.AddInt32(val.(*int32), -1) == 0 {
						wg.Add(1)
						l.readyCh <- up
					}
				}
			}
		}(app)
	}

	wg.Wait()
	if err := firstErr.Load(); err != nil {
		return err.(error)
	}

	return nil
}
