package core

import (
	"fmt"
	"math"
)

type Size struct {
	InBytes uint64
	Value   string
	Unit    string
}

func (s *Size) Convert() {
	bf := float64(s.InBytes)
	for _, unit := range []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi"} {
		if math.Abs(bf) < 1024.0 {
			s.Unit = fmt.Sprintf("%sB", unit)
			s.Value = fmt.Sprintf("%3.1f", bf)
			return
		}
		bf /= 1024.0
	}
}
