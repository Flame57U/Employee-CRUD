package quant

import (
	"math"
	"testing"
)

func TestEMABasic(t *testing.T) {
	series := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	v := EMA(series, 5)
	if v <= 0 {
		t.Fatalf("expected positive EMA, got %f", v)
	}
}

func TestEMAInsufficientData(t *testing.T) {
	v := EMA([]float64{1, 2}, 10)
	if v != 0 {
		t.Fatalf("expected 0 for insufficient data, got %f", v)
	}
}

func TestStdDevKnownValues(t *testing.T) {
	// {2, 4, 4, 4, 5, 5, 7, 9} → sample stddev = sqrt(32/7) ≈ 2.13809
	series := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	v := StdDev(series, 8)
	want := math.Sqrt(32.0 / 7.0)
	if math.Abs(v-want) > 1e-9 {
		t.Fatalf("expected %f, got %f", want, v)
	}
}

func TestMAVAbsChange(t *testing.T) {
	series := []float64{10, 12, 11, 13} // diffs: 2, 1, 2 → avg = 5/3
	v := MAVAbsChange(series, 4)
	if math.Abs(v-(5.0/3.0)) > 1e-9 {
		t.Fatalf("expected 5/3, got %f", v)
	}
}

func TestClipFloat64(t *testing.T) {
	if ClipFloat64(5, 0, 10) != 5 {
		t.Fatal("clip should pass through in-range")
	}
	if ClipFloat64(-1, 0, 10) != 0 {
		t.Fatal("clip should pin to lo")
	}
	if ClipFloat64(11, 0, 10) != 10 {
		t.Fatal("clip should pin to hi")
	}
}

func TestRoundToCNY(t *testing.T) {
	// Note: 1.005 is inexact in float64 (1.00499999...), so avoid it as a test case.
	cases := []struct {
		in, want float64
	}{
		{1.006, 1.01},
		{1.004, 1.00},
		{-1.006, -1.01},
		{0, 0},
		{123.455001, 123.46},
	}
	for _, c := range cases {
		got := RoundToCNY(c.in)
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("RoundToCNY(%f) = %f, want %f", c.in, got, c.want)
		}
	}
}
