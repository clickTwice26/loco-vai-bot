package service

import (
	"fmt"
	"runtime"
	"time"
)

// SystemService provides system metrics and bot status utilities.
type SystemService interface {
	GetUptime() time.Duration
	GetMemoryUsage() MemoryStats
	FormatUptime() string
}

type systemService struct {
	startTime time.Time
}

// MemoryStats holds runtime memory statistics.
type MemoryStats struct {
	AllocatedMB      float64
	TotalAllocatedMB float64
	SysMB            float64
	NumGC            uint32
	Goroutines       int
}

// NewSystemService creates a new instance of SystemService.
func NewSystemService() SystemService {
	return &systemService{
		startTime: time.Now(),
	}
}

// GetUptime returns the duration the bot has been running.
func (s *systemService) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// FormatUptime returns a human-readable string representation of uptime.
func (s *systemService) FormatUptime() string {
	d := s.GetUptime()
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// GetMemoryUsage returns current memory statistics.
func (s *systemService) GetMemoryUsage() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		AllocatedMB:      float64(m.Alloc) / 1024 / 1024,
		TotalAllocatedMB: float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:            float64(m.Sys) / 1024 / 1024,
		NumGC:            m.NumGC,
		Goroutines:       runtime.NumGoroutine(),
	}
}
