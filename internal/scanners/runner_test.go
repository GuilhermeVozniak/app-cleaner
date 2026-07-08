package scanners

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guhcostan/app-cleaner/internal/core"
)

type fakeScanner struct {
	cat  core.Category
	scan func(ctx context.Context, opts Options) core.ScanResult
}

func (f fakeScanner) Category() core.Category { return f.cat }

func (f fakeScanner) Scan(ctx context.Context, opts Options) core.ScanResult {
	return f.scan(ctx, opts)
}

func (f fakeScanner) Clean(ctx context.Context, items []core.CleanableItem, dryRun bool, progress core.ProgressFunc) core.CleanResult {
	return cleanWithFsx(f.cat, ctx, items, dryRun, progress)
}

func TestRunScansTotalsPanicIsolationAndProgress(t *testing.T) {
	goodCat := core.Category{ID: "fake-good", Name: "Fake Good"}
	badCat := core.Category{ID: "fake-bad", Name: "Fake Bad"}
	registerFake(t, fakeScanner{cat: goodCat, scan: func(ctx context.Context, opts Options) core.ScanResult {
		return core.ScanResult{Category: goodCat, Items: []core.CleanableItem{
			{Path: "/x/a", Size: 100, Name: "a"},
			{Path: "/x/b", Size: 50, Name: "b"},
		}, TotalSize: 150}
	}})
	registerFake(t, fakeScanner{cat: badCat, scan: func(ctx context.Context, opts Options) core.ScanResult {
		panic("boom")
	}})

	type call struct{ completed, total int }
	var calls []call // safe: RunScans invokes onResult serially under its lock
	sum := RunScans(context.Background(), []core.CategoryID{"fake-good", "fake-bad"}, Options{}, 2,
		func(completed, total int, r core.ScanResult) {
			calls = append(calls, call{completed, total})
		})

	if len(sum.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(sum.Results))
	}
	if sum.Results[0].Category.ID != "fake-good" || sum.Results[1].Category.ID != "fake-bad" {
		t.Fatalf("results not in input id order: %s, %s",
			sum.Results[0].Category.ID, sum.Results[1].Category.ID)
	}
	if !strings.Contains(sum.Results[1].Error, "boom") {
		t.Fatalf("panicking scanner Error = %q, want it to mention the panic value", sum.Results[1].Error)
	}
	if sum.Results[1].Category != badCat {
		t.Fatalf("panicking scanner must keep its Category, got %+v", sum.Results[1].Category)
	}
	if sum.TotalSize != 150 || sum.TotalItems != 2 {
		t.Fatalf("TotalSize=%d TotalItems=%d, want 150 and 2", sum.TotalSize, sum.TotalItems)
	}
	if len(calls) != 2 {
		t.Fatalf("onResult called %d times, want 2", len(calls))
	}
	for i, c := range calls {
		if c.completed != i+1 || c.total != 2 {
			t.Fatalf("progress call %d = %+v, want completed=%d total=2 (monotonic)", i, c, i+1)
		}
	}
}

func TestRunScansUnknownCategory(t *testing.T) {
	sum := RunScans(context.Background(), []core.CategoryID{"no-such"}, Options{}, 4, nil)
	if len(sum.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(sum.Results))
	}
	r := sum.Results[0]
	if r.Category.ID != "no-such" || !strings.Contains(r.Error, "unknown scanner category") {
		t.Fatalf("result = %+v, want Category.ID preserved and unknown-category error", r)
	}
}

func TestRunScansHonorsConcurrencyLimit(t *testing.T) {
	var active, maxActive int32
	mk := func(id string) fakeScanner {
		cat := core.Category{ID: core.CategoryID(id)}
		return fakeScanner{cat: cat, scan: func(ctx context.Context, opts Options) core.ScanResult {
			n := atomic.AddInt32(&active, 1)
			for {
				m := atomic.LoadInt32(&maxActive)
				if n <= m || atomic.CompareAndSwapInt32(&maxActive, m, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return core.ScanResult{Category: cat}
		}}
	}
	ids := []core.CategoryID{"fake-c1", "fake-c2", "fake-c3"}
	for _, id := range ids {
		registerFake(t, mk(string(id)))
	}
	RunScans(context.Background(), ids, Options{}, 1, nil)
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent scanners = %d, want 1", got)
	}
}
