package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	versionUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_version_updates_total",
			Help: "Total number of version updates by entity type",
		},
		[]string{"entity_type"},
	)
)

type Version struct {
	ID         string    `json:"id"`
	EntityID   string    `json:"entity_id"`
	EntityType string    `json:"entity_type"`
	Version    int       `json:"version"`
	Changes    []Change  `json:"changes"`
	CreatedAt  time.Time `json:"created_at"`
}

type Change struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

type VersionService struct {
	mu         sync.RWMutex
	versions   map[string][]Version
	lastUpdate time.Time
}

func NewVersionService() *VersionService {
	return &VersionService{
		versions:   make(map[string][]Version),
		lastUpdate: time.Now(),
	}
}

func (s *VersionService) CreateVersion(ctx context.Context, entityID string, entityType string, changes []Change) (*Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentVersions := s.versions[entityID]
	newVersion := Version{
		ID:         fmt.Sprintf("%s_%d", entityID, len(currentVersions)+1),
		EntityID:   entityID,
		EntityType: entityType,
		Version:    len(currentVersions) + 1,
		Changes:    changes,
		CreatedAt:  time.Now(),
	}

	s.versions[entityID] = append(s.versions[entityID], newVersion)
	versionUpdates.WithLabelValues(entityType).Inc()

	return &newVersion, nil
}

func (s *VersionService) GetVersions(ctx context.Context, entityID string) ([]Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[entityID]
	if !exists {
		return nil, fmt.Errorf("no versions found for entity %s", entityID)
	}

	return versions, nil
}

func (s *VersionService) GetLatestVersion(ctx context.Context, entityID string) (*Version, error) {
	versions, err := s.GetVersions(ctx, entityID)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for entity %s", entityID)
	}

	return &versions[len(versions)-1], nil
}

func (s *VersionService) CompareVersions(ctx context.Context, entityID string, v1, v2 int) ([]Change, error) {
	versions, err := s.GetVersions(ctx, entityID)
	if err != nil {
		return nil, err
	}

	if v1 < 1 || v2 < 1 || v1 > len(versions) || v2 > len(versions) {
		return nil, fmt.Errorf("invalid version numbers")
	}

	var changes []Change
	for i := v1; i <= v2; i++ {
		changes = append(changes, versions[i-1].Changes...)
	}

	return changes, nil
}

func (s *VersionService) CleanupOldVersions(ctx context.Context, maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for entityID, versions := range s.versions {
		var newVersions []Version
		for _, v := range versions {
			if v.CreatedAt.After(cutoff) {
				newVersions = append(newVersions, v)
			}
		}
		s.versions[entityID] = newVersions
	}

	return nil
}

func (s *VersionService) GetEntityChanges(ctx context.Context, entityID string, start, end time.Time) ([]Change, error) {
	versions, err := s.GetVersions(ctx, entityID)
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, v := range versions {
		if (v.CreatedAt.After(start) || v.CreatedAt.Equal(start)) &&
			(v.CreatedAt.Before(end) || v.CreatedAt.Equal(end)) {
			changes = append(changes, v.Changes...)
		}
	}

	return changes, nil
}
