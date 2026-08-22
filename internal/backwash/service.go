package backwash

import (
	"context"
	"fmt"

	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

type Service struct {
	probe *media.Probe
}

func NewService(maxClog float64) *Service {
	return &Service{probe: media.NewProbe(maxClog, 0.35, 0.55, 1.0)}
}

func (s *Service) EnsureMedia(ctx context.Context, profile model.MediaProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.probe.Evaluate(profile); err != nil {
		return fmt.Errorf("backwash service filter=%s: %w", profile.FilterID, err)
	}
	return nil
}
