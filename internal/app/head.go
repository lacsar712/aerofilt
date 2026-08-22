package app

import (
	"fmt"

	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/model"
)

func (a *App) EnsureHeadReady(sample model.HeadSample) error {
	if err := a.levelMon.Validate(sample, a.cfg.MinHeadM, a.cfg.MaxHeadM); err != nil {
		return fmt.Errorf("app head cell=%s: %w", sample.CellID, err)
	}
	return nil
}

func (a *App) ClassifyHead(err error) string {
	return level.Classify(err)
}
