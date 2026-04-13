package metrics

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Snapshot struct {
	TotalRequests  int64
	Completed      int64
	Successes      int64
	Failures       int64
	Retries        int64
	TotalLatencyNs int64
	MinLatencyNs   int64
	MaxLatencyNs   int64
	LatenciesNs    []int64
	Duration       time.Duration
	StatusCodes    map[int]int64
	ErrorSummary   map[string]int64
	FailedSamples  []FailedSample
}

type FailedSample struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

type Metrics struct {
	totalRequests  int64
	completed      atomic.Int64
	successes      atomic.Int64
	failures       atomic.Int64
	retries        atomic.Int64
	totalLatencyNs atomic.Int64
	minLatencyNs   atomic.Int64
	maxLatencyNs   atomic.Int64
	latencyIndex   atomic.Uint64
	latenciesNs    []int64
	mu             sync.Mutex
	statusCodes    [600]atomic.Int64
	errorSummary   map[string]int64
	failedSamples  []FailedSample
}

func New(totalRequests int) *Metrics {
	sampleSize := min(max(totalRequests, 1), 4096)
	return &Metrics{
		totalRequests: int64(totalRequests),
		latenciesNs:   make([]int64, sampleSize),
		errorSummary:  make(map[string]int64),
		failedSamples: make([]FailedSample, 0, min(totalRequests, 25)),
	}
}

func (m *Metrics) Record(latency time.Duration, success bool) {
	m.completed.Add(1)
	latencyNs := latency.Nanoseconds()
	m.totalLatencyNs.Add(latencyNs)
	m.updateMinLatency(latencyNs)
	m.updateMaxLatency(latencyNs)

	if len(m.latenciesNs) > 0 {
		index := m.latencyIndex.Add(1) - 1
		m.latenciesNs[index%uint64(len(m.latenciesNs))] = latencyNs
	}

	if success {
		m.successes.Add(1)
		return
	}

	m.failures.Add(1)
}

func (m *Metrics) Snapshot(duration time.Duration) Snapshot {
	completed := m.completed.Load()
	sampleCount := completed
	if sampleCount > int64(len(m.latenciesNs)) {
		sampleCount = int64(len(m.latenciesNs))
	}

	latencies := make([]int64, sampleCount)
	copy(latencies, m.latenciesNs[:sampleCount])
	slices.Sort(latencies)

	m.mu.Lock()
	errorSummary := make(map[string]int64, len(m.errorSummary))
	for key, count := range m.errorSummary {
		errorSummary[key] = count
	}

	failedSamples := make([]FailedSample, len(m.failedSamples))
	copy(failedSamples, m.failedSamples)
	m.mu.Unlock()

	statusCodes := make(map[int]int64)
	for code := range m.statusCodes {
		if count := m.statusCodes[code].Load(); count > 0 {
			statusCodes[code] = count
		}
	}

	return Snapshot{
		TotalRequests:  m.totalRequests,
		Completed:      completed,
		Successes:      m.successes.Load(),
		Failures:       m.failures.Load(),
		Retries:        m.retries.Load(),
		TotalLatencyNs: m.totalLatencyNs.Load(),
		MinLatencyNs:   m.minLatencyNs.Load(),
		MaxLatencyNs:   m.maxLatencyNs.Load(),
		LatenciesNs:    latencies,
		Duration:       duration,
		StatusCodes:    statusCodes,
		ErrorSummary:   errorSummary,
		FailedSamples:  failedSamples,
	}
}

func (m *Metrics) AddRetries(count int) {
	if count <= 0 {
		return
	}

	m.retries.Add(int64(count))
}

func (m *Metrics) RecordStatusCode(statusCode int) {
	if statusCode <= 0 || statusCode >= len(m.statusCodes) {
		return
	}

	m.statusCodes[statusCode].Add(1)
}

func (m *Metrics) RecordFailure(statusCode int, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if statusCode > 0 && statusCode < len(m.statusCodes) {
		m.statusCodes[statusCode].Add(1)
	}

	if errMsg != "" {
		m.errorSummary[errMsg]++
	}

	if len(m.failedSamples) < cap(m.failedSamples) {
		m.failedSamples = append(m.failedSamples, FailedSample{
			StatusCode: statusCode,
			Error:      errMsg,
		})
	}
}

func (s Snapshot) AverageLatency() time.Duration {
	if s.Completed == 0 {
		return 0
	}

	return time.Duration(s.TotalLatencyNs / s.Completed)
}

func (s Snapshot) MinLatency() time.Duration {
	return time.Duration(s.MinLatencyNs)
}

func (s Snapshot) MaxLatency() time.Duration {
	return time.Duration(s.MaxLatencyNs)
}

func (s Snapshot) Throughput() float64 {
	seconds := s.Duration.Seconds()
	if seconds == 0 {
		return 0
	}

	return float64(s.Completed) / seconds
}

func (s Snapshot) SuccessRate() float64 {
	if s.Completed == 0 {
		return 0
	}

	return float64(s.Successes) / float64(s.Completed) * 100
}

func (s Snapshot) ErrorRate() float64 {
	if s.Completed == 0 {
		return 0
	}

	return float64(s.Failures) / float64(s.Completed) * 100
}

func (s Snapshot) Percentile(percentile float64) time.Duration {
	if len(s.LatenciesNs) == 0 {
		return 0
	}

	if percentile <= 0 {
		return time.Duration(s.LatenciesNs[0])
	}

	if percentile >= 100 {
		return time.Duration(s.LatenciesNs[len(s.LatenciesNs)-1])
	}

	position := int64((percentile / 100) * float64(len(s.LatenciesNs)-1))
	return time.Duration(s.LatenciesNs[position])
}

func (s Snapshot) Report() string {
	const (
		colorReset = "\033[0m"
		colorRed   = "\033[31m"
		colorGreen = "\033[32m"
		colorCyan  = "\033[36m"
		colorBold  = "\033[1m"
	)

	formatRow := func(label, value string) string {
		return fmt.Sprintf("  %-20s %s\n", label, value)
	}

	colorizeCount := func(value int64, color string) string {
		return fmt.Sprintf("%s%d%s", color, value, colorReset)
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("%s%sLoad Test Summary%s\n", colorBold, colorCyan, colorReset))
	report.WriteString(strings.Repeat("=", 52))
	report.WriteByte('\n')
	report.WriteString(fmt.Sprintf("%sRequests%s\n", colorBold, colorReset))
	report.WriteString(formatRow("Total", fmt.Sprintf("%d", s.TotalRequests)))
	report.WriteString(formatRow("Completed", fmt.Sprintf("%d", s.Completed)))
	report.WriteString(formatRow("Successful", colorizeCount(s.Successes, colorGreen)))
	report.WriteString(formatRow("Failed", colorizeCount(s.Failures, colorRed)))
	report.WriteString(formatRow("Retry Attempts", fmt.Sprintf("%d", s.Retries)))
	report.WriteByte('\n')
	report.WriteString(fmt.Sprintf("%sLatency%s\n", colorBold, colorReset))
	report.WriteString(formatRow("Min", s.MinLatency().Round(time.Microsecond).String()))
	report.WriteString(formatRow("Average", s.AverageLatency().Round(time.Microsecond).String()))
	report.WriteString(formatRow("Max", s.MaxLatency().Round(time.Microsecond).String()))
	report.WriteString(formatRow("P50", s.Percentile(50).Round(time.Microsecond).String()))
	report.WriteString(formatRow("P90", s.Percentile(90).Round(time.Microsecond).String()))
	report.WriteString(formatRow("P95", s.Percentile(95).Round(time.Microsecond).String()))
	report.WriteString(formatRow("P99", s.Percentile(99).Round(time.Microsecond).String()))
	report.WriteByte('\n')
	report.WriteString(fmt.Sprintf("%sRun%s\n", colorBold, colorReset))
	report.WriteString(formatRow("TPS", fmt.Sprintf("%.2f req/s", s.Throughput())))
	report.WriteString(formatRow("Success Rate", fmt.Sprintf("%.2f%%", s.SuccessRate())))
	report.WriteString(formatRow("Error Rate", fmt.Sprintf("%.2f%%", s.ErrorRate())))
	report.WriteString(formatRow("Duration", s.Duration.Round(time.Millisecond).String()))

	if len(s.StatusCodes) > 0 {
		report.WriteByte('\n')
		report.WriteString(fmt.Sprintf("%sStatus Codes%s\n", colorBold, colorReset))
		for _, code := range sortedStatusCodes(s.StatusCodes) {
			report.WriteString(formatRow(fmt.Sprintf("%d", code), fmt.Sprintf("%d", s.StatusCodes[code])))
		}
	}

	if len(s.ErrorSummary) > 0 {
		report.WriteByte('\n')
		report.WriteString(fmt.Sprintf("%sError Summary%s\n", colorBold, colorReset))
		for _, key := range sortedErrorKeys(s.ErrorSummary) {
			label := key
			if len(label) > 32 {
				label = strings.TrimSpace(label[:29]) + "..."
			}
			report.WriteString(formatRow(label, fmt.Sprintf("%d", s.ErrorSummary[key])))
		}
	}

	return report.String()
}

func (s Snapshot) AverageLatencyMilliseconds() float64 {
	return float64(s.AverageLatency()) / float64(time.Millisecond)
}

func (s Snapshot) PercentileMilliseconds(percentile float64) float64 {
	return float64(s.Percentile(percentile)) / float64(time.Millisecond)
}

func sortedStatusCodes(values map[int]int64) []int {
	codes := make([]int, 0, len(values))
	for code := range values {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

func sortedErrorKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func (m *Metrics) updateMinLatency(latencyNs int64) {
	for {
		current := m.minLatencyNs.Load()
		if current != 0 && latencyNs >= current {
			return
		}
		if m.minLatencyNs.CompareAndSwap(current, latencyNs) {
			return
		}
	}
}

func (m *Metrics) updateMaxLatency(latencyNs int64) {
	for {
		current := m.maxLatencyNs.Load()
		if latencyNs <= current {
			return
		}
		if m.maxLatencyNs.CompareAndSwap(current, latencyNs) {
			return
		}
	}
}
