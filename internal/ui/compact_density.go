package ui

import "time"

const compactDenseRowViewportWidth = 48
const compactUltraDenseRowViewportWidth = 40

type compactRowDensity int

const (
	compactRowDensityNormal compactRowDensity = iota
	compactRowDensityDense
	compactRowDensityUltra
)

func compactAccountRowDensity(limit int) compactRowDensity {
	switch {
	case limit > 0 && limit <= compactUltraDenseRowViewportWidth:
		return compactRowDensityUltra
	case limit > 0 && limit <= compactDenseRowViewportWidth:
		return compactRowDensityDense
	default:
		return compactRowDensityNormal
	}
}

func (density compactRowDensity) usesRelativeReset() bool {
	return density >= compactRowDensityDense
}

func compactDenseBarMaxWidth(density compactRowDensity) int {
	if density == compactRowDensityUltra {
		return 14
	}
	return 16
}

func compactDenseResetText(resetAt time.Time) string {
	if resetAt.IsZero() {
		return "unknown"
	}

	remaining := time.Until(resetAt)
	if remaining <= 0 {
		return "now"
	}
	return compactRemainingUnit(remaining)
}
