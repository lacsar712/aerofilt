package filter

import (
	"errors"
	"fmt"
)

// Guard is the cross-package façade that re-wraps over-temperature so
// errors.Is(err, ErrOverTemp) still selects the vent procedure.
type Guard struct {
	book  *Book
	limit float64
}

// NewGuard binds a filter book to a hard temperature limit.
func NewGuard(book *Book, limitC float64) *Guard {
	return &Guard{book: book, limit: limitC}
}

// Evaluate checks over-temperature and preserves the sentinel chain with %w.
func (g *Guard) Evaluate() error {
	if g == nil || g.book == nil {
		return fmt.Errorf("temperature guard not configured")
	}
	err := g.book.CheckOverTemp(g.limit)
	if err != nil {
		return fmt.Errorf("temperature guard: %w", err)
	}
	return nil
}

// Procedure maps an error to the operator procedure code.
func Procedure(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrOverTemp):
		return "vent"
	default:
		return "generic"
	}
}
