package scanners

import (
	"context"
	"fmt"
	"sync"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

// RunScans runs the scanners for ids in parallel, bounded by a semaphore of
// size concurrency (clamped to 1..len(ids)). Admission is FIFO: scanners
// start in ids order, so concurrency=1 runs them strictly sequentially (the
// CLI's serial mode). Results[i] always corresponds to ids[i]. A scanner
// that panics — or an id with no registered scanner — yields
// ScanResult{Category, Error}; the batch never aborts. onResult (may be nil)
// is invoked after each scanner finishes, serially under the runner's lock,
// with completed = 1..total strictly increasing.
func RunScans(ctx context.Context, ids []core.CategoryID, opts Options, concurrency int,
	onResult func(completed, total int, r core.ScanResult),
) core.ScanSummary {
	total := len(ids)
	if total == 0 {
		return core.ScanSummary{Results: []core.ScanResult{}}
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > total {
		concurrency = total
	}

	results := make([]core.ScanResult, total)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for i, id := range ids {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int, id core.CategoryID) {
			defer wg.Done()
			defer func() { <-sem }()

			r := runOne(ctx, id, opts)

			mu.Lock()
			defer mu.Unlock()
			results[i] = r
			completed++
			if onResult != nil {
				onResult(completed, total, r)
			}
		}(i, id)
	}
	wg.Wait()

	summary := core.ScanSummary{Results: results}
	for _, r := range results {
		summary.TotalSize += r.TotalSize
		summary.TotalItems += len(r.Items)
	}
	return summary
}

// runOne executes a single scanner with panic isolation.
func runOne(ctx context.Context, id core.CategoryID, opts Options) (r core.ScanResult) {
	s, ok := Get(id)
	if !ok {
		return core.ScanResult{
			Category: core.Category{ID: id},
			Items:    []core.CleanableItem{},
			Error:    fmt.Sprintf("unknown scanner category: %s", id),
		}
	}
	defer func() {
		if rec := recover(); rec != nil {
			r = core.ScanResult{
				Category: s.Category(),
				Items:    []core.CleanableItem{},
				Error:    fmt.Sprintf("scanner panicked: %v", rec),
			}
		}
	}()
	return s.Scan(ctx, opts)
}
