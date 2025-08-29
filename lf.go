package lifecyclex

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/skyrocket-qy/erx"
)

type Closer func(context.Context) error

type SimpleLifecycle struct {
	closers []Closer
}

func NewSimpleLifecycle() *SimpleLifecycle {
	return &SimpleLifecycle{}
}

func (l *SimpleLifecycle) Add(fn Closer) {
	l.closers = append(l.closers, fn)
}

func (l *SimpleLifecycle) Shutdown(c context.Context) error {
	for i := len(l.closers) - 1; i >= 0; i-- {
		if err := l.closers[i](c); err != nil {
			return erx.W(err)
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

func (l *LifecycleParallel) computeDegree() {
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
	l.computeDegree()

	var (
		wg       sync.WaitGroup
		firstErr atomic.Value
	)

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
				if err := closer(c); err != nil {
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
		if err, ok := err.(error); ok {
			return erx.W(err)
		}

		return erx.Newf(erx.ErrUnknown, "failed to shutdown: %v", err)
	}

	return nil
}
