package repo

import "sync"

// flight coalesces concurrent work for the same key so that only one goroutine
// performs it while the rest wait and observe the same outcome. It mirrors
// golang.org/x/sync/singleflight but is kept tiny and dependency-free: proxy
// fetches for the same coordinate collapse into one upstream round-trip, which
// is the difference between a single GET and an upstream rate-limit (429) storm
// when many build modules resolve the same BOM/POM at once.
type flight struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	wg  sync.WaitGroup
	res fetchOutcome
}

func newFlight() *flight { return &flight{calls: map[string]*flightCall{}} }

// do runs fn for key unless an identical call is already in progress, in which
// case it waits for that call and returns its result. The shared result is then
// applied independently by each caller to its own response writer.
func (f *flight) do(key string, fn func() fetchOutcome) fetchOutcome {
	f.mu.Lock()
	if c, ok := f.calls[key]; ok {
		f.mu.Unlock()
		c.wg.Wait()
		return c.res
	}
	c := &flightCall{}
	c.wg.Add(1)
	f.calls[key] = c
	f.mu.Unlock()

	c.res = fn()

	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	c.wg.Done()
	return c.res
}
