package service

import (
	"testing"
	"time"
)

func TestSystemService(t *testing.T) {
	sys := NewSystemService()

	// Small pause to ensure non-zero uptime
	time.Sleep(10 * time.Millisecond)

	uptime := sys.GetUptime()
	if uptime <= 0 {
		t.Errorf("expected positive uptime, got %v", uptime)
	}

	formatted := sys.FormatUptime()
	if formatted == "" {
		t.Errorf("expected non-empty formatted uptime")
	}

	mem := sys.GetMemoryUsage()
	if mem.Goroutines <= 0 {
		t.Errorf("expected at least 1 goroutine, got %d", mem.Goroutines)
	}
}
