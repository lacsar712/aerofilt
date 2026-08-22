package filter

import (
	"errors"
	"fmt"

	"github.com/lacsar712/aerofilt/internal/blower"
	"github.com/lacsar712/aerofilt/internal/level"
	"github.com/lacsar712/aerofilt/internal/media"
	"github.com/lacsar712/aerofilt/internal/model"
)

type Guard struct {
	book    *Book
	level   *level.Monitor
	media   *media.Probe
	blower  *blower.Controller
	minHead float64
	maxHead float64
}

func NewGuard(book *Book, lvl *level.Monitor, med *media.Probe, bl *blower.Controller, minHead, maxHead float64) *Guard {
	return &Guard{book: book, level: lvl, media: med, blower: bl, minHead: minHead, maxHead: maxHead}
}

func (g *Guard) Evaluate(cell model.CellID, profile model.MediaProfile, sample model.HeadSample) error {
	if err := g.level.Validate(sample, g.minHead, g.maxHead); err != nil {
		if level.IsLow(err) || level.IsHigh(err) {
			return err
		}
		return fmt.Errorf("head: %w", err)
	}
	if err := g.media.Evaluate(profile); err != nil {
		if media.IsClogged(err) {
			return err
		}
		return fmt.Errorf("media: %w", err)
	}
	c, ok := g.book.Cell(cell)
	if !ok {
		return model.ErrInvalidSample
	}
	if !c.Online {
		return model.ErrCellOffline
	}
	if g.blower.Phase() == model.BlowerTripped {
		return fmt.Errorf("%w", model.ErrBlowerTripped)
	}
	return nil
}

func Classify(err error) string {
	switch {
	case errors.Is(err, model.ErrPlantEStop):
		return "estop"
	case level.IsLow(err):
		return "raise_level"
	case level.IsHigh(err):
		return "lower_level"
	case media.IsClogged(err):
		return "backwash"
	case blower.IsTripped(err):
		return "reset_blower"
	default:
		return "hold"
	}
}
