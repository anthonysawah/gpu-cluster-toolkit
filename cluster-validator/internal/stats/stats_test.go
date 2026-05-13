package stats

import (
	"math"
	"testing"
	"time"
)

func TestMean_Basic(t *testing.T) {
	ds := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
	}
	got := Mean(ds)
	want := 20 * time.Millisecond
	if got != want {
		t.Errorf("Mean = %v, want %v", got, want)
	}
}

func TestMean_Empty(t *testing.T) {
	if got := Mean(nil); got != 0 {
		t.Errorf("Mean(nil) = %v, want 0", got)
	}
}

func TestStdDev_KnownValues(t *testing.T) {
	// Values: 2, 4, 4, 4, 5, 5, 7, 9 (classic stats textbook example)
	// Mean = 5, population stddev = 2
	ds := []time.Duration{2, 4, 4, 4, 5, 5, 7, 9}
	got := StdDev(ds)
	want := time.Duration(2)
	// Allow 1ns tolerance for float rounding.
	if math.Abs(float64(got-want)) > 1 {
		t.Errorf("StdDev = %v, want %v (±1ns)", got, want)
	}
}

func TestStdDev_SingleValue(t *testing.T) {
	if got := StdDev([]time.Duration{42 * time.Millisecond}); got != 0 {
		t.Errorf("StdDev of single value = %v, want 0", got)
	}
}

func TestStdDev_Empty(t *testing.T) {
	if got := StdDev(nil); got != 0 {
		t.Errorf("StdDev(nil) = %v, want 0", got)
	}
}