package store

import (
	"sync"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type FilterStore struct {
	mu      sync.RWMutex
	plantID model.PlantID
	mode    model.PlantMode
	cells   map[model.CellID]model.FilterCell
	media      map[model.FilterID]model.MediaProfile
	mediaCache []model.MediaProfile
}

func NewFilterStore(plant model.PlantID, specs []model.FilterCell, profiles []model.MediaProfile) *FilterStore {
	cells := make(map[model.CellID]model.FilterCell, len(specs))
	for _, c := range specs {
		cells[c.ID] = c
	}
	media := make(map[model.FilterID]model.MediaProfile, len(profiles))
	for _, p := range profiles {
		media[p.FilterID] = p
	}
	return &FilterStore{plantID: plant, mode: model.PlantModeStandby, cells: cells, media: media}
}

func (s *FilterStore) SetMode(mode model.PlantMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
}

func (s *FilterStore) Mode() model.PlantMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *FilterStore) UpdateCell(cell model.FilterCell) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cells[cell.ID] = cell
}

func (s *FilterStore) UpdateMedia(profile model.MediaProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.media[profile.FilterID] = profile
}

func (s *FilterStore) Cell(id model.CellID) (model.FilterCell, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cells[id]
	return c, ok
}

func (s *FilterStore) Snapshot(at time.Time) model.FilterSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cells := make([]model.FilterCell, 0, len(s.cells))
	for _, c := range s.cells {
		cells = append(cells, c)
	}
	if s.mediaCache == nil {
		s.mediaCache = make([]model.MediaProfile, 0, len(s.media))
		for _, m := range s.media {
			s.mediaCache = append(s.mediaCache, m)
		}
	}
	return model.FilterSnapshot{PlantID: s.plantID, Mode: s.mode, Cells: cells, Media: s.mediaCache, TakenAt: at, HeadAvgM: model.AvgHead(cells)}
}

func (s *FilterStore) ReplaceAll(cells []model.FilterCell, media []model.MediaProfile, mode model.PlantMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.cells = make(map[model.CellID]model.FilterCell, len(cells))
	for _, c := range cells {
		s.cells[c.ID] = c
	}
	s.media = make(map[model.FilterID]model.MediaProfile, len(media))
	for _, m := range media {
		s.media[m.FilterID] = m
	}
}
