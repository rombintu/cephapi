package core

import (
	"fmt"
	"math"
)

type Size struct {
	InBytes float64
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

func (s *Size) PlusBytes(count float64) {
	s.InBytes += count
}

func (s *Size) Set(count float64) {
	s.InBytes = count
	s.Convert()
}

func Percent(percent int, all int) float64 {
	return ((float64(all) * float64(percent)) / float64(100))
}
