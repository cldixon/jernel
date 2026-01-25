package daemon

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// PeriodToDuration converts a rate period string to a time.Duration
func PeriodToDuration(period string) (time.Duration, error) {
	switch period {
	case "hour":
		return time.Hour, nil
	case "day":
		return 24 * time.Hour, nil
	case "week":
		return 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid rate_period: %s (must be hour, day, or week)", period)
	}
}

// CalculateNextInterval returns a random duration for the next trigger
// Based on rolling randomness: random value between 0.5x and 1.5x the average interval
func CalculateNextInterval(rate int, period string) (time.Duration, error) {
	if rate <= 0 {
		return 0, fmt.Errorf("rate must be positive, got %d", rate)
	}

	periodDuration, err := PeriodToDuration(period)
	if err != nil {
		return 0, err
	}

	// Average interval between entries
	avgInterval := periodDuration / time.Duration(rate)

	// Random interval between 0.5x and 1.5x average
	minInterval := avgInterval / 2
	maxInterval := avgInterval + avgInterval/2

	// Calculate random offset within range
	rangeSize := maxInterval - minInterval
	if rangeSize <= 0 {
		return avgInterval, nil
	}

	// Use crypto/rand for better randomness
	randomOffset, err := cryptoRandDuration(rangeSize)
	if err != nil {
		// Fallback to average if random fails
		return avgInterval, nil
	}

	return minInterval + randomOffset, nil
}

// cryptoRandDuration returns a random duration between 0 and max
func cryptoRandDuration(max time.Duration) (time.Duration, error) {
	if max <= 0 {
		return 0, nil
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return time.Duration(n.Int64()), nil
}

// CalculateNextTrigger returns the time for the next entry trigger
func CalculateNextTrigger(rate int, period string) (time.Time, error) {
	interval, err := CalculateNextInterval(rate, period)
	if err != nil {
		return time.Time{}, err
	}

	return time.Now().Add(interval), nil
}
