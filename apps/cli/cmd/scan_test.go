package cmd

import (
	"testing"

	"github.com/GuilhermeVozniak/app-cleaner/packages/engine/core"
)

func TestResolveCategoriesDefaultsToAllWhenEmpty(t *testing.T) {
	ids, unknown := resolveCategories(nil)
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v, want none", unknown)
	}
	want := core.CategoriesInOrder()
	if len(ids) != len(want) {
		t.Fatalf("len(ids) = %d, want %d (all categories)", len(ids), len(want))
	}
	for i, cat := range want {
		if ids[i] != cat.ID {
			t.Fatalf("ids[%d] = %q, want %q (stable order)", i, ids[i], cat.ID)
		}
	}
}

func TestResolveCategoriesWarnsAndSkipsUnknown(t *testing.T) {
	ids, unknown := resolveCategories([]string{"trash", "no-such-category", "downloads"})
	if len(unknown) != 1 || unknown[0] != "no-such-category" {
		t.Fatalf("unknown = %v, want [no-such-category]", unknown)
	}
	want := []core.CategoryID{"trash", "downloads"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}
