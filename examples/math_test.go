package math

import (
	"testing"

)


func TestAdd(t *testing.T) {

	t.Run("Add_two_positive_integers", func(t *testing.T) {
		// Setup
		arg0 := 5
		arg1 := 3
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != 8 {
			t.Errorf("expected: expected %v, got %v", 8, result)
		}

	})

	t.Run("Add_two_negative_integers", func(t *testing.T) {
		// Setup
		arg0 := -7
		arg1 := -2
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != -9 {
			t.Errorf("expected: expected %v, got %v", -9, result)
		}

	})

	t.Run("Add_zero_and_a_positive_integer", func(t *testing.T) {
		// Setup
		arg0 := 0
		arg1 := 4
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != 4 {
			t.Errorf("expected: expected %v, got %v", 4, result)
		}

	})

	t.Run("Add_zero_and_a_negative_integer", func(t *testing.T) {
		// Setup
		arg0 := 0
		arg1 := -6
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != -6 {
			t.Errorf("expected: expected %v, got %v", -6, result)
		}

	})

	t.Run("Add_maximum_safe_integer_values", func(t *testing.T) {
		// Setup
		arg0 := 1000000
		arg1 := 999999
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != 2000000 {
			t.Errorf("expected: expected %v, got %v", 2000000, result)
		}

	})

	t.Run("Add_minimum_safe_integer_values", func(t *testing.T) {
		// Setup
		arg0 := -1000000
		arg1 := -999999
		
		// Act
		result := Add(arg0, arg1)

		// Assert

		if result != -2000000 {
			t.Errorf("expected: expected %v, got %v", -2000000, result)
		}

	})

}

func TestMultiply(t *testing.T) {

	t.Run("Multiply_positive_numbers", func(t *testing.T) {
		// Setup
		arg0 := 2
		arg1 := 3
		
		// Act
		result := Multiply(arg0, arg1)

		// Assert

		if result != 6 {
			t.Errorf("result: expected %v, got %v", 6, result)
		}

	})

	t.Run("Multiply_negative_and_positive_numbers", func(t *testing.T) {
		// Setup
		arg0 := -2
		arg1 := 3
		
		// Act
		result := Multiply(arg0, arg1)

		// Assert

		if result != -6 {
			t.Errorf("result: expected %v, got %v", -6, result)
		}

	})

	t.Run("Multiply_zero_with_a_number", func(t *testing.T) {
		// Setup
		arg0 := 0
		arg1 := 5
		
		// Act
		result := Multiply(arg0, arg1)

		// Assert

		if result != 0 {
			t.Errorf("result: expected %v, got %v", 0, result)
		}

	})

	t.Run("Multiply_two_negative_numbers", func(t *testing.T) {
		// Setup
		arg0 := -4
		arg1 := -3
		
		// Act
		result := Multiply(arg0, arg1)

		// Assert

		if result != 12 {
			t.Errorf("result: expected %v, got %v", 12, result)
		}

	})

}

