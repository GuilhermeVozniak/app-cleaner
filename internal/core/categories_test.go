package core

import "testing"

func TestCategoriesRegistryComplete(t *testing.T) {
	if len(Categories) != 16 {
		t.Fatalf("Categories has %d entries, want 16", len(Categories))
	}
	cases := []struct {
		id                    CategoryID
		name                  string
		group                 CategoryGroup
		description           string
		safety                SafetyLevel
		note                  string
		supportsFileSelection bool
	}{
		{"system-cache", "User Cache Files", "System Junk", "Application caches stored in ~/Library/Caches", "moderate", "Some apps may need to rebuild cache on next launch", false},
		{"system-logs", "System Log Files", "System Junk", "System and application logs", "moderate", "Logs may be useful for debugging issues", false},
		{"temp-files", "Temporary Files", "System Junk", "Temporary files in /tmp and /var/folders", "safe", "", false},
		{"trash", "Trash", "Storage", "Files in the Trash bin", "safe", "", false},
		{"downloads", "Old Downloads", "Storage", "Downloads older than 30 days", "risky", "May contain important files you forgot about", true},
		{"browser-cache", "Browser Cache", "Browsers", "Cache from Chrome, Safari, Firefox, and Arc", "safe", "", false},
		{"dev-cache", "Development Cache", "Development", "npm, yarn, pip, Xcode DerivedData, CocoaPods cache", "moderate", "Projects will need to rebuild/reinstall dependencies", false},
		{"homebrew", "Homebrew Cache", "Development", "Homebrew download cache and old versions", "safe", "", false},
		{"docker", "Docker", "Development", "Unused Docker images, containers, and volumes", "safe", "", false},
		{"ios-backups", "iOS Backups", "Storage", "iPhone and iPad backup files", "risky", "DANGER: You may lose important device backups permanently!", false},
		{"mail-attachments", "Mail Attachments", "Storage", "Downloaded email attachments from Mail.app", "risky", "May contain important documents and files", false},
		{"language-files", "Language Files", "System Junk", "Unused language localizations in applications", "risky", "May break apps if you switch system language", false},
		{"large-files", "Large Files", "Large Files", "Files larger than 500MB for review", "risky", "Review each file carefully before deleting", true},
		{"node-modules", "Node Modules", "Development", "Orphaned node_modules in old projects", "moderate", "Projects will need npm install to restore", false},
		{"duplicates", "Duplicate Files", "Storage", "Files with identical content", "risky", "Review carefully - keeps newest copy by default", false},
		{"launch-agents", "Orphaned Launch Agents", "System Junk", "Launch agents pointing to non-existent applications", "moderate", "Removing launch agents will prevent applications from auto-starting. Only orphaned items (pointing to non-existent apps) are detected.", false},
	}
	for _, c := range cases {
		t.Run(string(c.id), func(t *testing.T) {
			got, ok := Categories[c.id]
			if !ok {
				t.Fatalf("Categories[%q] missing", c.id)
			}
			want := Category{
				ID:                    c.id,
				Name:                  c.name,
				Group:                 c.group,
				Description:           c.description,
				SafetyLevel:           c.safety,
				SafetyNote:            c.note,
				SupportsFileSelection: c.supportsFileSelection,
			}
			if got != want {
				t.Errorf("Categories[%q] =\n  %+v\nwant\n  %+v", c.id, got, want)
			}
		})
	}
}

func TestCategoriesInOrder(t *testing.T) {
	want := []CategoryID{
		"system-cache", "system-logs", "temp-files", "trash", "downloads",
		"browser-cache", "dev-cache", "homebrew", "docker", "ios-backups",
		"mail-attachments", "language-files", "large-files", "node-modules",
		"duplicates", "launch-agents",
	}
	got := CategoriesInOrder()
	if len(got) != len(want) {
		t.Fatalf("CategoriesInOrder returned %d categories, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("CategoriesInOrder()[%d].ID = %q, want %q", i, got[i].ID, id)
		}
		if got[i] != Categories[id] {
			t.Errorf("CategoriesInOrder()[%d] does not match Categories[%q]", i, id)
		}
	}
}
