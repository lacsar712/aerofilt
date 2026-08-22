package model

import "time"

func CloneFilterSnapshot(s FilterSnapshot) FilterSnapshot {
	out := s
	out.Cells = make([]FilterCell, len(s.Cells))
	copy(out.Cells, s.Cells)
	out.Media = make([]MediaProfile, len(s.Media))
	copy(out.Media, s.Media)
	return out
}

func AvgHead(cells []FilterCell) float64 {
	var sum float64
	var n int
	for _, c := range cells {
		if !c.Online {
			continue
		}
		sum += c.HeadM
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func CellsInHeadBand(cells []FilterCell, targetM, bandM float64) (bool, []CellID) {
	var bad []CellID
	for _, c := range cells {
		if !c.Online {
			continue
		}
		diff := c.HeadM - targetM
		if diff < 0 {
			diff = -diff
		}
		if diff > bandM {
			bad = append(bad, c.ID)
		}
	}
	return len(bad) == 0, bad
}

func ZeroTime() time.Time { return time.Time{} }
