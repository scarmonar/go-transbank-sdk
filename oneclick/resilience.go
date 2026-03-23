package oneclick

import (
	"sync"
	"sync/atomic"
	"time"
)

// ClientMetrics tracks core operational indicators for the SDK.
type ClientMetrics struct {
	requestsTotal        uint64
	requestErrors        uint64
	confirmationFailures uint64
	totalLatencyNanos    uint64
}

// MetricsSnapshot is a point-in-time, read-only metrics view.
type MetricsSnapshot struct {
	RequestsTotal        uint64
	RequestErrors        uint64
	ConfirmationFailures uint64
	ErrorRate            float64
	AverageLatency       time.Duration
}

func (m *ClientMetrics) record(latency time.Duration, failed bool, confirmationFailed bool) {
	atomic.AddUint64(&m.requestsTotal, 1)
	atomic.AddUint64(&m.totalLatencyNanos, uint64(latency))
	if failed {
		atomic.AddUint64(&m.requestErrors, 1)
	}
	if confirmationFailed {
		atomic.AddUint64(&m.confirmationFailures, 1)
	}
}

func (m *ClientMetrics) incConfirmationFailure() {
	atomic.AddUint64(&m.confirmationFailures, 1)
}

func (m *ClientMetrics) snapshot() MetricsSnapshot {
	total := atomic.LoadUint64(&m.requestsTotal)
	errors := atomic.LoadUint64(&m.requestErrors)
	confirmFailures := atomic.LoadUint64(&m.confirmationFailures)
	totalLatency := atomic.LoadUint64(&m.totalLatencyNanos)

	snapshot := MetricsSnapshot{
		RequestsTotal:        total,
		RequestErrors:        errors,
		ConfirmationFailures: confirmFailures,
	}
	if total > 0 {
		snapshot.ErrorRate = float64(errors) / float64(total)
		snapshot.AverageLatency = time.Duration(totalLatency / total)
	}
	return snapshot
}

type circuitBreaker struct {
	policy CircuitBreakerPolicy
	clock  Clock

	mu       sync.Mutex
	failures int
	openTill time.Time
}

func newCircuitBreaker(policy CircuitBreakerPolicy, clock Clock) *circuitBreaker {
	return &circuitBreaker{
		policy: policy,
		clock:  clock,
	}
}

func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openTill.IsZero() {
		return true
	}

	now := b.clock.Now()
	if now.After(b.openTill) || now.Equal(b.openTill) {
		b.openTill = time.Time{}
		b.failures = 0
		return true
	}

	return false
}

func (b *circuitBreaker) markSuccess() {
	b.mu.Lock()
	b.failures = 0
	b.openTill = time.Time{}
	b.mu.Unlock()
}

func (b *circuitBreaker) markFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	if b.failures >= b.policy.FailureThreshold {
		b.openTill = b.clock.Now().Add(b.policy.Cooldown)
	}
}
