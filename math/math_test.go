package math_test

import (
	"testing"

	"github.com/vukyn/kuery/math"
)

// Handles positive number correctly
func TestAbsPositiveNumber(t *testing.T) {
	input := 5
	expected := 5
	result := math.Abs(input)
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Handles negative number correctly
func TestAbsNegativeNumber(t *testing.T) {
	input := -1
	expected := 1
	result := math.Abs(input)
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Returns the smallest number from a list of integers
func TestReturnsSmallestNumber(t *testing.T) {
	result := math.Min(3, 1, 4, 1, 5, 9)
	expected := 1
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Handles a single element list correctly
func TestSingleElementList(t *testing.T) {
	result := math.Min(42)
	expected := 42
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Returns the maximum value from a list of integers
func TestMaxReturnsMaximumValue(t *testing.T) {
	result := math.Max(1, 3, 2, 5, 4)
	expected := 5
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Handles an empty list gracefully
func TestMaxHandlesEmptyList(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for empty list, but did not get one")
		}
	}()
	_ = math.Max[int]()
}

// Summing a list of integers
func TestSumIntegers(t *testing.T) {
	result := math.Sum(1, 2, 3, 4, 5)
	expected := 15

	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Summing an empty list
func TestSumEmptyList(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for empty list, but did not get one")
		}
	}()

	_ = math.Sum[int]()
}

// Computes product of a list of integers
func TestProductOfIntegers(t *testing.T) {
	result := math.Product(1, 2, 3, 4)
	expected := 24
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Single element in the list
func TestProductSingleElement(t *testing.T) {
	result := math.Product(5)
	expected := 5
	if result != expected {
		t.Errorf("Expected %d, but got %d", expected, result)
	}
}

// Computes product of a list of floating-point numbers
func TestProductOfFloatingPointNumbers(t *testing.T) {
	result := math.Product(1.5, 2.5, 3.5)
	expected := 13.125
	if result != expected {
		t.Errorf("Expected %f, but got %f", expected, result)
	}
}
