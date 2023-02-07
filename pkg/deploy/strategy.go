package deploy

import (
	"time"
)

// Strategy determines how hosts are updated during a deployment.
type Strategy struct {
	name      string
	batchSize int
	delay     time.Duration
}

// NewStrategy creates a deployment strategy from the strategy name.
func NewStrategy(name string) *Strategy {
	return &Strategy{
		name:      name,
		batchSize: 1,
		delay:     10 * time.Second,
	}
}

// WithBatchSize sets the number of hosts to update simultaneously.
func (s *Strategy) WithBatchSize(size int) *Strategy {
	if size < 1 {
		size = 1
	}
	s.batchSize = size
	return s
}

// WithDelay sets the delay between batches.
func (s *Strategy) WithDelay(d time.Duration) *Strategy {
	s.delay = d
	return s
}

// Plan divides hosts into batches based on the strategy.
func (s *Strategy) Plan(hosts []string) [][]string {
	switch s.name {
	case "blue-green":
		return s.planBlueGreen(hosts)
	default:
		return s.planRolling(hosts)
	}
}

// planRolling creates batches of the configured batch size.
func (s *Strategy) planRolling(hosts []string) [][]string {
	var batches [][]string

	for i := 0; i < len(hosts); i += s.batchSize {
		end := i + s.batchSize
		if end > len(hosts) {
			end = len(hosts)
		}
		batches = append(batches, hosts[i:end])
	}

	return batches
}

// planBlueGreen splits hosts into two groups for blue-green deployment.
// The first half gets updated first, then once healthy, the second half follows.
func (s *Strategy) planBlueGreen(hosts []string) [][]string {
	if len(hosts) < 2 {
		return [][]string{hosts}
	}

	mid := len(hosts) / 2
	return [][]string{
		hosts[:mid],
		hosts[mid:],
	}
}

// BatchDelay returns the configured delay between batches.
func (s *Strategy) BatchDelay() time.Duration {
	return s.delay
}

// Name returns the strategy name.
func (s *Strategy) Name() string {
	return s.name
}

// IsBlueGreen returns true if this is a blue-green deployment strategy.
func (s *Strategy) IsBlueGreen() bool {
	return s.name == "blue-green"
}
