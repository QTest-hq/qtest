package math

import (
	"testing"

)


func TestMathCombined(t *testing.T) {
	
	var result interface{}
	_ = result
	
	// Add positive numbers
	result = Add(2, 3)
	
	if result != 5 {
		t.Errorf("expected %v, got %v", 5, result)
	}
	
	
	// Add negative numbers
	result = Add(-1, -1)
	
	if result != -2 {
		t.Errorf("expected %v, got %v", -2, result)
	}
	
	
	// Add zero and positive number
	result = Add(0, 5)
	
	if result != 5 {
		t.Errorf("expected %v, got %v", 5, result)
	}
	
	
	// Add zero and negative number
	result = Add(0, -3)
	
	if result != -3 {
		t.Errorf("expected %v, got %v", -3, result)
	}
	
	
	// Add maximum int32 value and one (no overflow on 64-bit)
	result = Add(2147483647, 1)

	if result != 2147483648 {
		t.Errorf("expected %v, got %v", 2147483648, result)
	}
	
	
	// Add minimum int32 value and negative one (no underflow on 64-bit)
	result = Add(-2147483648, -1)

	if result != -2147483649 {
		t.Errorf("expected %v, got %v", -2147483649, result)
	}
	
	
	// Add maximum and minimum int values
	result = Add(2147483647, -2147483648)
	
	if result != -1 {
		t.Errorf("expected %v, got %v", -1, result)
	}
	
	
	// Multiply positive numbers
	result = Multiply(2, 3)
	
	if result != 6 {
		t.Errorf("expected %v, got %v", 6, result)
	}
	
	
	// Multiply negative and positive numbers
	result = Multiply(-2, 3)
	
	if result != -6 {
		t.Errorf("expected %v, got %v", -6, result)
	}
	
	
	// Multiply two negative numbers
	result = Multiply(-2, -3)
	
	if result != 6 {
		t.Errorf("expected %v, got %v", 6, result)
	}
	
	
	// Multiply zero and positive number
	result = Multiply(0, 5)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply zero and negative number
	result = Multiply(0, -5)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply positive number and zero
	result = Multiply(5, 0)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply negative number and zero
	result = Multiply(-5, 0)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply maximum int value with positive number
	result = Multiply(2147483647, 2)
	
	if result != 4294967294 {
		t.Errorf("expected %v, got %v", 4294967294, result)
	}
	
	
	// Multiply minimum int value with negative number
	result = Multiply(-2147483648, -2)
	
	if result != 4294967296 {
		t.Errorf("expected %v, got %v", 4294967296, result)
	}
	
	
	// Multiply maximum int value with negative number
	result = Multiply(2147483647, -2)
	
	if result != -4294967294 {
		t.Errorf("expected %v, got %v", -4294967294, result)
	}
	
	
	// Multiply minimum int value with positive number
	result = Multiply(-2147483648, 2)
	
	if result != -4294967296 {
		t.Errorf("expected %v, got %v", -4294967296, result)
	}
	
	
	// Multiply maximum int value with zero
	result = Multiply(2147483647, 0)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply minimum int value with zero
	result = Multiply(-2147483648, 0)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
}

