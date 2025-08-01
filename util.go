package selectel

import (
	"fmt"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type SeqMatcher[T any] struct {
	t      *testing.T
	values []T
}

func (m SeqMatcher[T]) Matches(x any) bool {
	seq, ok := x.(iter.Seq[T])
	if !ok {
		return false
	}

	expected := m.values
	actual := slices.Collect(seq)
	return assert.ElementsMatch(m.t, expected, actual)
}

func (m SeqMatcher[T]) String() string {
	return fmt.Sprintf("%+v", m.values)
}

func getTTL(values ...time.Duration) time.Duration {
	var result time.Duration
	for _, value := range values {
		if result < time.Minute || (value > time.Minute && value < result) {
			result = value
		}
	}

	return max(result, time.Minute)
}
