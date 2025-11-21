package math

import (
	"testing"

)


func TestMathCombined(t *testing.T) {
	
	var result interface{}
	_ = result
	
	// Add positive numbers
	result = Add(5, 3)
	
	if result != 8 {
		t.Errorf("expected %v, got %v", 8, result)
	}
	
	
	// Add negative numbers
	result = Add(-2, -4)
	
	if result != -6 {
		t.Errorf("expected %v, got %v", -6, result)
	}
	
	
	// Add zero and positive number
	result = Add(0, 7)
	
	if result != 7 {
		t.Errorf("expected %v, got %v", 7, result)
	}
	
	
	// Add zero and negative number
	result = Add(0, -3)
	
	if result != -3 {
		t.Errorf("expected %v, got %v", -3, result)
	}
	
	
	// Multiply positive numbers
	result = Multiply(5, 3)
	
	if result != 15 {
		t.Errorf("expected %v, got %v", 15, result)
	}
	
	
	// Multiply negative and positive numbers
	result = Multiply(-4, 6)
	
	if result != -24 {
		t.Errorf("expected %v, got %v", -24, result)
	}
	
	
	// Multiply zero with positive number
	result = Multiply(0, 7)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Multiply two negative numbers
	result = Multiply(-3, -2)
	
	if result != 6 {
		t.Errorf("expected %v, got %v", 6, result)
	}
	
	
}

