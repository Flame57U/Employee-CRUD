package quant

import (
	"math"
	"testing"
)

func TestStepNeutralSignalReturnsHalfWeight(t *testing.T) {
	e := SigmoidEngine{Beta: 2.0, Gamma: 0, MinRebalanceThreshold: 0}

	targetWeight, _ := e.Step(0, 0.5)
	if math.Abs(targetWeight-0.5) > 1e-9 {
		t.Fatalf("expected targetWeight ~= 0.5, got %f", targetWeight)
	}
}

func TestStepLargePositiveSignalReturnsLowWeight(t *testing.T) {
	e := SigmoidEngine{Beta: 1.0, Gamma: 0, MinRebalanceThreshold: 0}

	targetWeight, _ := e.Step(10, 0.5)
	if targetWeight >= 0.1 {
		t.Fatalf("expected targetWeight < 0.1, got %f", targetWeight)
	}
}

func TestStepLargeNegativeSignalReturnsHighWeight(t *testing.T) {
	e := SigmoidEngine{Beta: 1.0, Gamma: 0, MinRebalanceThreshold: 0}

	targetWeight, _ := e.Step(-10, 0.5)
	if targetWeight <= 0.9 {
		t.Fatalf("expected targetWeight > 0.9, got %f", targetWeight)
	}
}

func TestStepDeltaBelowThresholdReturnsZero(t *testing.T) {
	e := SigmoidEngine{Beta: 0, Gamma: 0, MinRebalanceThreshold: 0.05}

	_, delta := e.Step(0, 0.49)
	if delta != 0 {
		t.Fatalf("expected delta == 0, got %f", delta)
	}
}

func TestStepGammaAddsMeanReversionPressureAboveHalf(t *testing.T) {
	base := SigmoidEngine{Beta: 0, Gamma: 0, MinRebalanceThreshold: 0}
	withGamma := SigmoidEngine{Beta: 0, Gamma: 2.0, MinRebalanceThreshold: 0}

	_, baseDelta := base.Step(0, 0.8)
	_, gammaDelta := withGamma.Step(0, 0.8)

	if !(gammaDelta < baseDelta) {
		t.Fatalf("expected gamma delta (%f) < base delta (%f)", gammaDelta, baseDelta)
	}
	if !(gammaDelta < 0) {
		t.Fatalf("expected gamma delta to be negative, got %f", gammaDelta)
	}
}
