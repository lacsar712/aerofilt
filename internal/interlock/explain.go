// Package interlock provides human-readable interlock explanations.
package interlock

import (
	"fmt"
	"strings"

	"github.com/lacsar712/aerofilt/internal/model"
)

// Explain formats an interlock decision for operator display.
func Explain(d model.InterlockDecision) string {
	if d.Allowed {
		return "cycle permitted"
	}
	if len(d.Reasons) == 0 {
		return fmt.Sprintf("denied: %s", d.Code)
	}
	return fmt.Sprintf("%s — %s", d.Code, strings.Join(d.Reasons, "; "))
}

// Summary returns a one-line gate status.
func Summary(g *Gates) string {
	if g == nil {
		return "gates unavailable"
	}
	snap := g.SnapshotGates()
	var blocked []string
	for k, v := range snap {
		if k == "strict" {
			continue
		}
		if !v && k != "estop" {
			blocked = append(blocked, k)
		}
	}
	if snap["estop"] {
		return "ESTOP ACTIVE"
	}
	if len(blocked) == 0 {
		return "all gates ready"
	}
	return "blocked: " + strings.Join(blocked, ", ")
}
