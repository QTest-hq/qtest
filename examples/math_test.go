package math

import (
	"testing"

)


func TestAdd(t *testing.T) {

	t.Run("Adding_two_positive_numbers_returns_their_sum", func(t *testing.T) {
		// Setup
		a := 5
		b := 3
		
		// Act
		result := Add(a, b)

		// Assert

		if result != 8 {
			t.Errorf("result: expected %v, got %v", 8, result)
		}

	})

	t.Run("Adding_two_negative_numbers_returns_their_sum", func(t *testing.T) {
		// Setup
		a := -2
		b := -4
		
		// Act
		result := Add(a, b)

		// Assert

		if result != -6 {
			t.Errorf("result: expected %v, got %v", -6, result)
		}

	})

	t.Run("Adding_zero_returns_the_other_number_unchanged", func(t *testing.T) {
		// Setup
		a := 0
		b := 7
		
		// Act
		result := Add(a, b)

		// Assert

		if result != 7 {
			t.Errorf("result: expected %v, got %v", 7, result)
		}

	})

	t.Run("Adding_negative_zero_returns_the_other_number_unchanged", func(t *testing.T) {
		// Setup
		a := 0
		b := 7
		
		// Act
		result := Add(a, b)

		// Assert

		if result != 7 {
			t.Errorf("result: expected %v, got %v", 7, result)
		}

	})

}

func TestMultiply(t *testing.T) {

	t.Run("Multiplying_two_positive_numbers_returns_their_product", func(t *testing.T) {
		// Setup
		a := 5
		b := 3
		
		// Act
		result := Multiply(a, b)

		// Assert

		if result != 15 {
			t.Errorf("result: expected %v, got %v", 15, result)
		}

	})

	t.Run("Multiplying_two_negative_numbers_returns_positive_product", func(t *testing.T) {
		// Setup
		a := -2
		b := -4
		
		// Act
		result := Multiply(a, b)

		// Assert

		if result != 8 {
			t.Errorf("result: expected %v, got %v", 8, result)
		}

	})

	t.Run("Multiplying_any_number_with_zero_returns_zero", func(t *testing.T) {
		// Setup
		a := 0
		b := 7
		
		// Act
		result := Multiply(a, b)

		// Assert

		if result != 0 {
			t.Errorf("result: expected %v, got %v", 0, result)
		}

	})

	t.Run("Multiplying_a_negative_and_positive_number_returns_negative_product", func(t *testing.T) {
		// Setup
		a := -3
		b := 4
		
		// Act
		result := Multiply(a, b)

		// Assert

		if result != -12 {
			t.Errorf("result: expected %v, got %v", -12, result)
		}

	})

}

