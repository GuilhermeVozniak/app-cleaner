package core

// Categories is the registry of all 16 cleaning categories. IDs, names,
// groups, descriptions, safety levels, and safety notes are verbatim from
// the CLI (mac-cleaner-cli/src/types.ts); see docs/reference/porting-notes.json
// before changing any copy here.
var Categories = map[CategoryID]Category{
	"system-cache": {
		ID: "system-cache", Name: "User Cache Files", Group: "System Junk",
		Description: "Application caches stored in ~/Library/Caches",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Some apps may need to rebuild cache on next launch",
	},
	"system-logs": {
		ID: "system-logs", Name: "System Log Files", Group: "System Junk",
		Description: "System and application logs",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Logs may be useful for debugging issues",
	},
	"temp-files": {
		ID: "temp-files", Name: "Temporary Files", Group: "System Junk",
		Description: "Temporary files in /tmp and /var/folders",
		SafetyLevel: SafetySafe,
	},
	"trash": {
		ID: "trash", Name: "Trash", Group: "Storage",
		Description: "Files in the Trash bin",
		SafetyLevel: SafetySafe,
	},
	"downloads": {
		ID: "downloads", Name: "Old Downloads", Group: "Storage",
		Description: "Downloads older than 30 days",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May contain important files you forgot about",
		SupportsFileSelection: true,
	},
	"browser-cache": {
		ID: "browser-cache", Name: "Browser Cache", Group: "Browsers",
		Description: "Cache from Chrome, Safari, Firefox, and Arc",
		SafetyLevel: SafetySafe,
	},
	"dev-cache": {
		ID: "dev-cache", Name: "Development Cache", Group: "Development",
		Description: "npm, yarn, pip, Xcode DerivedData, CocoaPods cache",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Projects will need to rebuild/reinstall dependencies",
	},
	"homebrew": {
		ID: "homebrew", Name: "Homebrew Cache", Group: "Development",
		Description: "Homebrew download cache and old versions",
		SafetyLevel: SafetySafe,
	},
	"docker": {
		ID: "docker", Name: "Docker", Group: "Development",
		Description: "Unused Docker images, containers, and build cache",
		SafetyLevel: SafetySafe,
	},
	"ios-backups": {
		ID: "ios-backups", Name: "iOS Backups", Group: "Storage",
		Description: "iPhone and iPad backup files",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "DANGER: You may lose important device backups permanently!",
	},
	"mail-attachments": {
		ID: "mail-attachments", Name: "Mail Attachments", Group: "Storage",
		Description: "Downloaded email attachments from Mail.app",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May contain important documents and files",
	},
	"language-files": {
		ID: "language-files", Name: "Language Files", Group: "System Junk",
		Description: "Unused language localizations in applications",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "May break apps if you switch system language",
	},
	"large-files": {
		ID: "large-files", Name: "Large Files", Group: "Large Files",
		Description: "Files larger than 500MB for review",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "Review each file carefully before deleting",
		SupportsFileSelection: true,
	},
	"node-modules": {
		ID: "node-modules", Name: "Node Modules", Group: "Development",
		Description: "Orphaned node_modules in old projects",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Projects will need npm install to restore",
	},
	"duplicates": {
		ID: "duplicates", Name: "Duplicate Files", Group: "Storage",
		Description: "Files with identical content",
		SafetyLevel: SafetyRisky,
		SafetyNote:  "Review carefully - keeps newest copy by default",
	},
	"launch-agents": {
		ID: "launch-agents", Name: "Orphaned Launch Agents", Group: "System Junk",
		Description: "Launch agents pointing to non-existent applications",
		SafetyLevel: SafetyModerate,
		SafetyNote:  "Removing launch agents will prevent applications from auto-starting. Only orphaned items (pointing to non-existent apps) are detected.",
	},
}

// categoryOrder is the stable display order: the spec §5 table order, which
// is the CLI's declaration order. Grouping by CategoryGroup for display is a
// frontend concern.
var categoryOrder = []CategoryID{
	"system-cache", "system-logs", "temp-files", "trash", "downloads",
	"browser-cache", "dev-cache", "homebrew", "docker", "ios-backups",
	"mail-attachments", "language-files", "large-files", "node-modules",
	"duplicates", "launch-agents",
}

// CategoriesInOrder returns all 16 categories in stable display order.
func CategoriesInOrder() []Category {
	out := make([]Category, 0, len(categoryOrder))
	for _, id := range categoryOrder {
		out = append(out, Categories[id])
	}
	return out
}
