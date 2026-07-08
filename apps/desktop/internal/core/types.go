// Package core defines App Cleaner's shared domain types: the category
// registry, cleanable items, scan/clean result shapes, and size formatting.
// It never imports Wails and has no side effects. JSON tags mirror the
// original CLI's TypeScript field names so the frontend reuses the shapes.
package core

import "time"

type CategoryID string

type SafetyLevel string

const (
	SafetySafe     SafetyLevel = "safe"
	SafetyModerate SafetyLevel = "moderate"
	SafetyRisky    SafetyLevel = "risky"
)

type CategoryGroup string

type Category struct {
	ID                    CategoryID    `json:"id"`
	Name                  string        `json:"name"`
	Group                 CategoryGroup `json:"group"`
	Description           string        `json:"description"`
	SafetyLevel           SafetyLevel   `json:"safetyLevel"`
	SafetyNote            string        `json:"safetyNote,omitempty"`
	SupportsFileSelection bool          `json:"supportsFileSelection,omitempty"`
}

type CleanableItem struct {
	Path        string     `json:"path"`
	Size        int64      `json:"size"`
	Name        string     `json:"name"`
	IsDirectory bool       `json:"isDirectory"`
	ModifiedAt  *time.Time `json:"modifiedAt,omitempty"`
}

type ScanResult struct {
	Category  Category        `json:"category"`
	Items     []CleanableItem `json:"items"`
	TotalSize int64           `json:"totalSize"`
	Error     string          `json:"error,omitempty"`
}

type ScanSummary struct {
	Results    []ScanResult `json:"results"`
	TotalSize  int64        `json:"totalSize"`
	TotalItems int          `json:"totalItems"`
}

type CleanResult struct {
	Category     Category `json:"category"`
	CleanedItems int      `json:"cleanedItems"`
	FreedSpace   int64    `json:"freedSpace"`
	Errors       []string `json:"errors"`
}

type CleanSummary struct {
	Results           []CleanResult `json:"results"`
	TotalFreedSpace   int64         `json:"totalFreedSpace"`
	TotalCleanedItems int           `json:"totalCleanedItems"`
	TotalErrors       int           `json:"totalErrors"`
}

// ProgressFunc reports progress for long operations. current is 1-based and
// the callback fires BEFORE the item is processed (CLI parity).
type ProgressFunc func(current, total int, item CleanableItem)
