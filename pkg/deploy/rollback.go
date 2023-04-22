package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const versionDir = ".deployer"
const versionFile = "versions.json"

// VersionEntry records a deployment event.
type VersionEntry struct {
	Image     string    `json:"image"`
	Timestamp time.Time `json:"timestamp"`
}

// VersionTracker manages deployment version history for rollback support.
type VersionTracker struct {
	mu      sync.Mutex
	project string
	data    map[string][]VersionEntry
	path    string
}

// NewVersionTracker creates a new version tracker for the given project.
func NewVersionTracker(project string) *VersionTracker {
	t := &VersionTracker{
		project: project,
		data:    make(map[string][]VersionEntry),
		path:    filepath.Join(versionDir, versionFile),
	}
	t.load()
	return t
}

// Record stores a new deployment version for a service.
func (t *VersionTracker) Record(service, image string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry := VersionEntry{
		Image:     image,
		Timestamp: time.Now(),
	}

	t.data[service] = append(t.data[service], entry)

	// Keep only the last 20 versions per service
	if len(t.data[service]) > 20 {
		t.data[service] = t.data[service][len(t.data[service])-20:]
	}

	t.save()
}

// GetCurrent returns the most recently deployed image for a service.
func (t *VersionTracker) GetCurrent(service string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.data[service]
	if len(entries) == 0 {
		return ""
	}
	return entries[len(entries)-1].Image
}

// GetPrevious returns the image deployed before the current one.
func (t *VersionTracker) GetPrevious(service string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.data[service]
	if len(entries) < 2 {
		return ""
	}
	return entries[len(entries)-2].Image
}

// GetHistory returns all recorded images for a service, oldest first.
func (t *VersionTracker) GetHistory(service string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.data[service]
	images := make([]string, len(entries))
	for i, e := range entries {
		images[i] = e.Image
	}
	return images
}

// GetFullHistory returns all version entries for a service.
func (t *VersionTracker) GetFullHistory(service string) []VersionEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.data[service]
	result := make([]VersionEntry, len(entries))
	copy(result, entries)
	return result
}

// load reads the version file from disk.
func (t *VersionTracker) load() {
	data, err := os.ReadFile(t.path)
	if err != nil {
		return
	}

	var stored map[string][]VersionEntry
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	t.data = stored
}

// save writes the version data to disk.
func (t *VersionTracker) save() {
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create %s: %v\n", versionDir, err)
		return
	}

	data, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not marshal versions: %v\n", err)
		return
	}

	if err := os.WriteFile(t.path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", t.path, err)
	}
}
