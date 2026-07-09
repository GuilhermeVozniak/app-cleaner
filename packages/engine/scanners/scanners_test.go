package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestDefaultRoots(t *testing.T) {
	r := DefaultRoots()
	if r.Home == "" {
		t.Fatal("Home must not be empty")
	}
	if r.Tmp != "/tmp" || r.VarFolders != "/private/var/folders" || r.Applications != "/Applications" {
		t.Fatalf("unexpected roots: %+v", r)
	}
}

func TestGetUnknownCategory(t *testing.T) {
	if _, ok := Get("definitely-not-a-category"); ok {
		t.Fatal("Get must return ok=false for unknown ids")
	}
}

func TestAllReturnsRegisteredScannersInDisplayOrder(t *testing.T) {
	// Stays valid as scanner tasks land: All() must equal the registered
	// subset of core.CategoriesInOrder(), in that order.
	var want []core.CategoryID
	for _, c := range core.CategoriesInOrder() {
		if _, ok := registry[c.ID]; ok {
			want = append(want, c.ID)
		}
	}
	var got []core.CategoryID
	for _, s := range All() {
		got = append(got, s.Category().ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("All() ids = %v, want %v", got, want)
	}
}

func TestNewScanResult(t *testing.T) {
	cat := core.Categories["trash"]
	r := newScanResult(cat, nil)
	if r.Items == nil || len(r.Items) != 0 || r.TotalSize != 0 || r.Error != "" {
		t.Fatalf("empty result malformed: %+v", r)
	}
	r = newScanResult(cat, []core.CleanableItem{{Size: 7}, {Size: 5}})
	if r.TotalSize != 12 || r.Category.ID != "trash" {
		t.Fatalf("result = %+v, want TotalSize 12 for trash", r)
	}
}

func TestCleanWithFsxRealDelete(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	mkFile(t, f1, 10)
	items := []core.CleanableItem{
		{Path: f1, Size: 10, Name: "a.txt"},
		{Path: "/System/never-delete-me", Size: 5, Name: "never"}, // PROTECTED, no disk IO
	}
	var progressed []string
	res := cleanWithFsx(core.Categories["trash"], context.Background(), items, false,
		func(current, total int, it core.CleanableItem) {
			progressed = append(progressed, fmt.Sprintf("%d/%d %s", current, total, it.Name))
		})
	if res.Category.ID != "trash" {
		t.Fatalf("Category = %+v, want trash", res.Category)
	}
	if res.CleanedItems != 1 || res.FreedSpace != 10 {
		t.Fatalf("CleanedItems=%d FreedSpace=%d, want 1 and 10", res.CleanedItems, res.FreedSpace)
	}
	wantErrs := []string{"Failed to remove 1 items (1 PROTECTED)"}
	if !reflect.DeepEqual(res.Errors, wantErrs) {
		t.Fatalf("Errors = %v, want %v", res.Errors, wantErrs)
	}
	if _, err := os.Lstat(f1); !os.IsNotExist(err) {
		t.Fatal("a.txt should have been deleted")
	}
	if !reflect.DeepEqual(progressed, []string{"1/2 a.txt", "2/2 never"}) {
		t.Fatalf("progress calls = %v, want before-each-item 1-based calls", progressed)
	}
}

func TestCleanWithFsxDryRun(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	mkFile(t, f1, 10)
	items := []core.CleanableItem{
		{Path: f1, Size: 10, Name: "a.txt"},
		{Path: "/System/never-delete-me", Size: 5, Name: "never"},
	}
	res := cleanWithFsx(core.Categories["trash"], context.Background(), items, true, nil)
	if res.CleanedItems != 2 || res.FreedSpace != 15 || len(res.Errors) != 0 {
		t.Fatalf("dry run = %+v, want all items credited, no errors", res)
	}
	if _, err := os.Lstat(f1); err != nil {
		t.Fatal("dry run must not touch disk")
	}
}
