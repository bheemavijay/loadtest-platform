package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"loadtest/internal/generator"
	"loadtest/internal/metrics"
)

type RequestConfig struct {
	Method  string
	URL     string
	Headers http.Header
	Body    []byte
	RPS     int
	Retries int
}

type RateLimiter struct {
	tokens <-chan time.Time
}

type Engine struct {
	client  *http.Client
	metrics *metrics.Metrics
}

type RequestResult struct {
	Success    bool
	StatusCode int
	Error      string
}

func New(client *http.Client, metrics *metrics.Metrics) *Engine {
	return &Engine{
		client:  client,
		metrics: metrics,
	}
}

func (e *Engine) Run(ctx context.Context, requestConfig RequestConfig, totalRequests, concurrency int) error {
	return e.runRequests(ctx, requestConfig, totalRequests, concurrency, true)
}

func (e *Engine) Warmup(ctx context.Context, requestConfig RequestConfig, concurrency int, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	warmupCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return e.runUntilCanceled(warmupCtx, requestConfig, concurrency, false)
}

func (e *Engine) runWorker(ctx context.Context, requestConfig RequestConfig, limiter *RateLimiter, jobs <-chan struct{}) {
	e.runWorkerWithMode(ctx, requestConfig, limiter, jobs, true)
}

func (e *Engine) runWorkerWithMode(ctx context.Context, requestConfig RequestConfig, limiter *RateLimiter, jobs <-chan struct{}, recordMetrics bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-jobs:
			if !ok {
				return
			}

			if err := limiter.Wait(ctx); err != nil {
				return
			}

			start := time.Now()
			success, retries := e.doRequest(ctx, requestConfig)
			if recordMetrics && e.metrics != nil {
				e.metrics.AddRetries(retries)
			}
			if recordMetrics && e.metrics != nil {
				e.metrics.Record(time.Since(start), success)
			}
		}
	}
}

func (e *Engine) doRequest(ctx context.Context, requestConfig RequestConfig) (bool, int) {
	retries := 0

	for attempt := 0; attempt <= requestConfig.Retries; attempt++ {
		result := e.sendOnce(ctx, requestConfig)
		e.recordRequestResult(result)
		if result.Success {
			return true, retries
		}

		if attempt < requestConfig.Retries {
			retries++
		}
	}

	return false, retries
}

func (e *Engine) sendOnce(ctx context.Context, requestConfig RequestConfig) RequestResult {
	var body io.Reader
	if len(requestConfig.Body) > 0 {
		processedBody := generator.ReplacePlaceholders(string(requestConfig.Body))
		dynamicBody, err := generator.GenerateDynamicBody(processedBody)
		if err != nil {
			return RequestResult{Error: err.Error()}
		}
		body = bytes.NewReader(dynamicBody)
	}

	req, err := http.NewRequestWithContext(ctx, requestConfig.Method, requestConfig.URL, body)
	if err != nil {
		return RequestResult{Error: err.Error()}
	}

	req.Header = requestConfig.Headers.Clone()

	resp, err := e.client.Do(req)
	if err != nil {
		return RequestResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return RequestResult{
			StatusCode: resp.StatusCode,
			Error:      err.Error(),
		}
	}

	success := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest
	result := RequestResult{
		Success:    success,
		StatusCode: resp.StatusCode,
	}
	if !success {
		result.Error = fmt.Sprintf("unexpected status code %d", resp.StatusCode)
	}

	return result
}

func (e *Engine) recordRequestResult(result RequestResult) {
	if e.metrics == nil {
		return
	}

	if result.Success {
		e.metrics.RecordStatusCode(result.StatusCode)
		return
	}

	e.metrics.RecordFailure(result.StatusCode, result.Error)
}

func NewRateLimiter(ctx context.Context, concurrency, rps int) (*RateLimiter, error) {
	if rps <= 0 {
		return &RateLimiter{}, nil
	}

	if rps > int(time.Second) {
		return nil, fmt.Errorf("rps must be less than or equal to %d", int(time.Second))
	}

	interval := time.Second / time.Duration(rps)
	ticker := time.NewTicker(interval)
	tokens := make(chan time.Time, maxInt(concurrency, 1))

	go func() {
		defer ticker.Stop()
		defer close(tokens)

		for {
			select {
			case <-ctx.Done():
				return
			case tick := <-ticker.C:
				select {
				case tokens <- tick:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return &RateLimiter{tokens: tokens}, nil
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.tokens == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case _, ok := <-r.tokens:
		if !ok {
			return ctx.Err()
		}
		return nil
	}
}

func (e *Engine) runRequests(ctx context.Context, requestConfig RequestConfig, totalRequests, concurrency int, recordMetrics bool) error {
	jobs := make(chan struct{})
	var wg sync.WaitGroup

	limiter, err := NewRateLimiter(ctx, concurrency, requestConfig.RPS)
	if err != nil {
		return err
	}

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.runWorkerWithMode(ctx, requestConfig, limiter, jobs, recordMetrics)
		}()
	}

	for range totalRequests {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- struct{}{}:
		}
	}

	close(jobs)
	wg.Wait()
	return nil
}

func (e *Engine) runUntilCanceled(ctx context.Context, requestConfig RequestConfig, concurrency int, recordMetrics bool) error {
	jobs := make(chan struct{}, maxInt(concurrency, 1))
	var wg sync.WaitGroup

	limiter, err := NewRateLimiter(ctx, concurrency, requestConfig.RPS)
	if err != nil {
		return err
	}

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.runWorkerWithMode(ctx, requestConfig, limiter, jobs, recordMetrics)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			if err := ctx.Err(); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			return nil
		case jobs <- struct{}{}:
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
