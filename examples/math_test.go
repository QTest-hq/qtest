package math

import (
	"math"
	"testing"
)

func TestMathCombined(t *testing.T) {
	var result interface{}
	_ = result

	// Add two positive integers
	result = Add(2, 3)

	// Add zero to a positive integer
	result = Add(0, 7)

	// Add two negative integers
	result = Add(-4, -6)

	// Add a positive integer to zero
	result = Add(5, 0)

	// Add the maximum int value to itself (using actual constants)
	result = Add(math.MaxInt32, math.MaxInt32)

	// Add the minimum int value to itself
	result = Add(math.MinInt32, math.MinInt32)

	// Add a positive integer to the maximum int value
	result = Add(math.MaxInt32, 1)

	// Add a negative integer to the minimum int value
	result = Add(math.MinInt32, -1)

	// Add two very large positive integers
	result = Add(1000000000, 2000000000)

	// Add two very large negative integers
	result = Add(-1000000000, -2000000000)

	// Add a positive integer to the maximum int value and handle overflow
	result = Add(math.MaxInt32, 1)

	// Add a negative integer to the minimum int value and handle underflow
	result = Add(math.MinInt32, -1)

	// Add two very large integers with different signs
	result = Add(1000000000, -500000000)

	// Add two very small integers with different signs
	result = Add(-1, 1)
}
