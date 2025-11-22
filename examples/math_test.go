package math

import (
	"testing"

)


func TestMathCombined(t *testing.T) {
	
	var result interface{}
	_ = result
	
	// Add two positive numbers
	result = Add(2, 3)
	
	if result != 5 {
		t.Errorf("expected %v, got %v", 5, result)
	}
	
	
	// Add a negative number and a positive number
	result = Add(-1, 1)
	
	if result != 0 {
		t.Errorf("expected %v, got %v", 0, result)
	}
	
	
	// Add two negative numbers
	result = Add(-2, -3)
	
	if result != -5 {
		t.Errorf("expected %v, got %v", -5, result)
	}
	
	
	// Add zero and a positive number
	result = Add(0, 7)
	
	if result != 7 {
		t.Errorf("expected %v, got %v", 7, result)
	}
	
	
}

