package selectel

import (
	"time"
)

func getTTL(values ...time.Duration) time.Duration {
	var result time.Duration
	for _, value := range values {
		if result < time.Minute || (value > time.Minute && value < result) {
			result = value
		}
	}

	return max(result, time.Minute)
}

type Set[T comparable] map[T]bool

func SetOf[T comparable](values ...T) Set[T] {
	set := make(map[T]bool)
	for _, value := range values {
		set[value] = true
	}

	return set
}
