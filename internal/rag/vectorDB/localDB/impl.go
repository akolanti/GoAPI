package localDB

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// --- math ---

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(magA) * math.Sqrt(magB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

// --- persistence ---

func (s *store) persist() {
	s.mu.RLock()
	//only persist non-cache collections
	toSave := make(map[string]*collection)
	for k, v := range s.Collections {
		if k != semanticCacheDBName {
			toSave[k] = v
		}
	}
	s.mu.RUnlock()

	writeJSON(s.dbPath, toSave)
}

func (s *store) persistCache() {
	s.mu.RLock()
	cacheCol := s.Collections[semanticCacheDBName]
	s.mu.RUnlock()

	if cacheCol != nil {
		writeJSON(s.cachePath, map[string]*collection{semanticCacheDBName: cacheCol})
	}
}

func writeJSON(path string, data any) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		logger.Error("Failed to create directory", "path", dir, "error", err)
		return
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal data", "error", err)
		return
	}

	if err := os.WriteFile(path, bytes, 0600); err != nil {
		logger.Error("Failed to write file", "path", path, "error", err)
	}
}

func loadFromFile(path string, s *store) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("Failed to read file", "path", path, "error", err)
		}
		return
	}

	var loaded map[string]*collection
	if err := json.Unmarshal(data, &loaded); err != nil {
		logger.Error("Failed to unmarshal file", "path", path, "error", err)
		return
	}
	for k, v := range loaded {
		s.Collections[k] = v
	}
	logger.Info("Loaded data from file", "path", path, "collections", len(loaded))
}

func loadCacheFromFile(path string, s *store) {
	loadFromFile(path, s)
}

// --- helpers ---

func payloadString(payload map[string]any, key string) string {
	val, ok := payload[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", val)
}
