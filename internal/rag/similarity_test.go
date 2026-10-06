package rag

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	v1 := []float32{1.0, 0.0, 0.0}
	v2 := []float32{1.0, 0.0, 0.0}
	v3 := []float32{0.0, 1.0, 0.0}
	v4 := []float32{-1.0, 0.0, 0.0}

	// Identical
	sim := CosineSimilarity(v1, v2)
	if math.Abs(sim-1.0) > 1e-5 {
		t.Errorf("expected 1.0, got %f", sim)
	}

	// Orthogonal
	sim = CosineSimilarity(v1, v3)
	if math.Abs(sim-0.0) > 1e-5 {
		t.Errorf("expected 0.0, got %f", sim)
	}

	// Opposite
	sim = CosineSimilarity(v1, v4)
	if math.Abs(sim-(-1.0)) > 1e-5 {
		t.Errorf("expected -1.0, got %f", sim)
	}

	// Mismatched lengths
	sim = CosineSimilarity(v1, []float32{1.0})
	if sim != 0.0 {
		t.Errorf("expected 0.0 for mismatched length, got %f", sim)
	}
}
