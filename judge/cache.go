package judge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agent-ecosystem/skill-validator/types"
)

// CachedResult holds a scoring result with metadata for cache storage.
type CachedResult struct {
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	File        string          `json:"file"`
	Type        string          `json:"type"`
	ContentHash string          `json:"content_hash"`
	ScoredAt    time.Time       `json:"scored_at"`
	Scores      json.RawMessage `json:"scores"`
}

// CacheKey generates a deterministic cache key from provider, model, score type,
// skill context, and file path. Returns the first 16 hex characters of a SHA-256 hash.
// Using file path (not content) means editing a file and re-running overwrites the
// same cache entry rather than creating an orphan.
func CacheKey(provider, model, scoreType, skillContext, filePath string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%s", provider, model, scoreType, skillContext, filePath)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)[:16]
}

// ContentHash returns a SHA-256 hash of the content for invalidation checks.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)[:16]
}

// CacheDir returns the cache directory path for a given skill directory.
func CacheDir(skillDir string) string {
	return filepath.Join(skillDir, ".score_cache")
}

// GetCached reads a cached result by key. Returns nil, false if not found.
func GetCached(cacheDir, key string) (*CachedResult, bool) {
	path := filepath.Join(cacheDir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var result CachedResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	return &result, true
}

// SaveCache writes a result to the cache directory.
func SaveCache(cacheDir, key string, result *CachedResult) error {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache entry: %w", err)
	}

	path := filepath.Join(cacheDir, key+".json")
	return os.WriteFile(path, data, 0o600)
}

// ListCached reads all cached results from the cache directory.
// Results are sorted by ScoredAt (most recent first).
func ListCached(cacheDir string) ([]*CachedResult, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache directory: %w", err)
	}

	var results []*CachedResult
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			continue
		}

		var result CachedResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		results = append(results, &result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ScoredAt.After(results[j].ScoredAt)
	})

	return results, nil
}

// FilterByModel returns only results matching the given model name.
func FilterByModel(results []*CachedResult, model string) []*CachedResult {
	var filtered []*CachedResult
	for _, r := range results {
		if r.Model == model {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// DeserializeScored unmarshals a CachedResult's Scores into the appropriate
// concrete type and returns it as a Scored interface. It uses the Type field
// to determine whether the result is a skill or reference score, falling back
// to checking File == "SKILL.md" for compatibility with older cache entries.
func DeserializeScored(r *CachedResult) (types.Scored, error) {
	if r.Type == "skill" || r.File == "SKILL.md" {
		var s SkillScores
		if err := json.Unmarshal(r.Scores, &s); err != nil {
			return nil, fmt.Errorf("deserializing skill scores: %w", err)
		}
		return &s, nil
	}
	var s RefScores
	if err := json.Unmarshal(r.Scores, &s); err != nil {
		return nil, fmt.Errorf("deserializing ref scores: %w", err)
	}
	return &s, nil
}

// LatestByFile returns the most recent cached result for each unique file,
// across all models. If model is non-empty, filters to that model first.
func LatestByFile(results []*CachedResult) map[string]*CachedResult {
	latest := make(map[string]*CachedResult)
	for _, r := range results {
		existing, ok := latest[r.File]
		if !ok || r.ScoredAt.After(existing.ScoredAt) {
			latest[r.File] = r
		}
	}
	return latest
}
