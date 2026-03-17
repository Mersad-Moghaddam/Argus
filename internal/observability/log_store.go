package observability

import (
	"sync"
	"time"

	"argus/internal/models"
)

// LogStore keeps recent runtime events in memory.
type LogStore struct {
	mu       sync.RWMutex
	logs     []models.SystemLog
	capacity int
}

// NewLogStore creates a bounded in-memory log store.
func NewLogStore(capacity int) *LogStore {
	if capacity < 50 {
		capacity = 50
	}
	return &LogStore{capacity: capacity, logs: make([]models.SystemLog, 0, capacity)}
}

// Add appends a new operational event.
func (s *LogStore) Add(level, source, action, message string, websiteID *int64, details map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := models.SystemLog{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Source:    source,
		Action:    action,
		Message:   message,
		WebsiteID: websiteID,
		Details:   details,
	}

	s.logs = append(s.logs, entry)
	if len(s.logs) > s.capacity {
		s.logs = s.logs[len(s.logs)-s.capacity:]
	}
}

// List returns newest logs first.
func (s *LogStore) List(limit int, websiteID *int64) []models.SystemLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > s.capacity {
		limit = s.capacity
	}

	result := make([]models.SystemLog, 0, limit)
	for i := len(s.logs) - 1; i >= 0; i-- {
		item := s.logs[i]
		if websiteID != nil {
			if item.WebsiteID == nil || *item.WebsiteID != *websiteID {
				continue
			}
		}
		result = append(result, item)
		if len(result) == limit {
			break
		}
	}
	return result
}
