package scanners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/guhcostan/app-cleaner/internal/core"
)

func TestMailAttachmentsScanner(t *testing.T) {
	s, ok := Get("mail-attachments")
	if !ok {
		t.Fatal("mail-attachments scanner not registered")
	}
	if s.Category() != core.Categories["mail-attachments"] {
		t.Fatalf("Category() = %+v, want core.Categories[mail-attachments]", s.Category())
	}

	opts := testOptions(t)
	if res := s.Scan(context.Background(), opts); res.Error != "" || len(res.Items) != 0 {
		t.Fatalf("missing dir must yield empty result without error, got %+v", res)
	}

	mail := filepath.Join(opts.Roots.Home, "Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	mkFile(t, filepath.Join(mail, "9F1A2B3C-0000-4444-8888-ABCDEF012345", "invoice.pdf"), 700)
	mkFile(t, filepath.Join(mail, "photo.jpg"), 300)

	res := s.Scan(context.Background(), opts)
	if res.Error != "" || len(res.Items) != 2 || res.TotalSize != 1000 {
		t.Fatalf("scan = %+v; want 2 items totalling 1000", res)
	}
	byName := map[string]core.CleanableItem{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	if d := byName["9F1A2B3C-0000-4444-8888-ABCDEF012345"]; !d.IsDirectory || d.Size != 700 {
		t.Fatalf("attachment folder = %+v; want dir of size 700, name kept as-is", d)
	}
	if f := byName["photo.jpg"]; f.IsDirectory || f.Size != 300 {
		t.Fatalf("file item = %+v; want file of size 300", f)
	}
}
