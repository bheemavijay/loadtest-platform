package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loadtest/internal/metrics"
	"loadtest/internal/worker"
	"gopkg.in/yaml.v3"
)

func main() {
	cfg, err := parseConfig()
	if err != nil {
		emitCLIError("configuration error", err.Error())
		os.Exit(1)
	}

	client := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: newTransport(cfg.Concurrency),
	}

	loadMetrics := metrics.New(cfg.TotalRequests)
	engine := worker.New(client, loadMetrics)
	requestConfig := worker.RequestConfig{
		Method:  cfg.Method,
		URL:     cfg.URL,
		Headers: cfg.Headers,
		Body:    []byte(cfg.JSONBody),
		RPS:     cfg.RPS,
		Retries: cfg.Retries,
	}

	if err := engine.Warmup(context.Background(), requestConfig, cfg.Concurrency, cfg.WarmupDuration); err != nil {
		emitCLIError("warm-up failed", err.Error())
		os.Exit(1)
	}

	start := time.Now()
	err = engine.Run(context.Background(), requestConfig, cfg.TotalRequests, cfg.Concurrency)
	duration := time.Since(start)

	if err != nil {
		emitCLIError("load test failed", err.Error())
		os.Exit(1)
	}

	snapshot := loadMetrics.Snapshot(duration)
	fmt.Print(snapshot.Report())

	if cfg.Output != "" {
		if err := exportResults(cfg, snapshot); err != nil {
			emitCLIError("failed to export results", err.Error())
			os.Exit(1)
		}
	}
}

type config struct {
	ConfigPath     string
	URL            string
	Method         string
	Headers        http.Header
	JSONBody       string
	RPS            int
	Retries        int
	WarmupDuration time.Duration
	TotalRequests  int
	Concurrency    int
	RequestTimeout time.Duration
	Output         string
}

func parseConfig() (config, error) {
	cfg := defaultConfig()

	configPath, err := parseConfigPath(os.Args[1:])
	if err != nil {
		return cfg, err
	}

	if configPath != "" {
		cfg, err = loadConfigFile(configPath)
		if err != nil {
			return cfg, err
		}
		cfg.ConfigPath = configPath
	}

	headerFlags := newHeaderFlag(cfg.Headers)
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "Path to a JSON or YAML config file")
	fs.StringVar(&cfg.URL, "url", cfg.URL, "Target URL for the load test")
	fs.StringVar(&cfg.Method, "method", cfg.Method, "HTTP method to use: GET or POST")
	fs.Var(headerFlags, "header", "Custom header in 'Key: Value' format; repeat flag for multiple headers")
	fs.StringVar(&cfg.JSONBody, "body", cfg.JSONBody, "JSON request body for POST requests")
	fs.IntVar(&cfg.RPS, "rps", cfg.RPS, "Target requests per second; 0 disables rate limiting")
	fs.IntVar(&cfg.Retries, "retries", cfg.Retries, "Number of retry attempts for failed requests")
	fs.DurationVar(&cfg.WarmupDuration, "warmup", cfg.WarmupDuration, "Warm-up duration before metrics collection; 0 disables warm-up")
	fs.IntVar(&cfg.TotalRequests, "requests", cfg.TotalRequests, "Total number of HTTP requests to send")
	fs.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "Number of concurrent workers")
	fs.DurationVar(&cfg.RequestTimeout, "timeout", cfg.RequestTimeout, "Per-request timeout")
	fs.StringVar(&cfg.Output, "output", cfg.Output, "Optional path to write the final results as JSON")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return cfg, err
	}

	if cfg.URL == "" {
		return cfg, fmt.Errorf("url is required")
	}

	parsedURL, err := url.ParseRequestURI(cfg.URL)
	if err != nil {
		return cfg, fmt.Errorf("invalid url: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("url must use http or https")
	}

	cfg.Method = normalizeMethod(cfg.Method)
	if cfg.Method != http.MethodGet && cfg.Method != http.MethodPost {
		return cfg, fmt.Errorf("method must be GET or POST")
	}

	if cfg.TotalRequests <= 0 {
		return cfg, fmt.Errorf("requests must be greater than zero")
	}

	if cfg.Concurrency <= 0 {
		return cfg, fmt.Errorf("concurrency must be greater than zero")
	}

	if cfg.Concurrency > cfg.TotalRequests {
		cfg.Concurrency = cfg.TotalRequests
	}

	if cfg.RequestTimeout <= 0 {
		return cfg, fmt.Errorf("timeout must be greater than zero")
	}

	if cfg.RPS < 0 {
		return cfg, fmt.Errorf("rps must be zero or greater")
	}

	if cfg.Retries < 0 {
		return cfg, fmt.Errorf("retries must be zero or greater")
	}

	if cfg.WarmupDuration < 0 {
		return cfg, fmt.Errorf("warmup must be zero or greater")
	}

	headers, err := headerFlags.Header()
	if err != nil {
		return cfg, err
	}
	cfg.Headers = headers

	if cfg.JSONBody != "" {
		if cfg.Method != http.MethodPost {
			return cfg, fmt.Errorf("body is only supported with POST requests")
		}

		if !json.Valid([]byte(cfg.JSONBody)) {
			return cfg, fmt.Errorf("body must be valid JSON")
		}

		if cfg.Headers.Get("Content-Type") == "" {
			cfg.Headers.Set("Content-Type", "application/json")
		}
	}

	return cfg, nil
}

func defaultConfig() config {
	return config{
		Method:         http.MethodGet,
		Headers:        make(http.Header),
		TotalRequests:  1000,
		Concurrency:    50,
		RequestTimeout: 10 * time.Second,
		WarmupDuration: 5 * time.Second,
	}
}

func parseConfigPath(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-config" || args[i] == "--config":
			if i+1 >= len(args) {
				return "", fmt.Errorf("config flag requires a path")
			}
			return args[i+1], nil
		case strings.HasPrefix(args[i], "-config="):
			return strings.TrimPrefix(args[i], "-config="), nil
		case strings.HasPrefix(args[i], "--config="):
			return strings.TrimPrefix(args[i], "--config="), nil
		}
	}

	return "", nil
}

func loadConfigFile(path string) (config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	raw := configFile{}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return cfg, fmt.Errorf("parse json config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return cfg, fmt.Errorf("parse yaml config: %w", err)
		}
	default:
		return cfg, fmt.Errorf("unsupported config format %q: use .json, .yaml, or .yml", filepath.Ext(path))
	}

	cfg.URL = raw.URL
	if raw.Method != "" {
		cfg.Method = raw.Method
	}
	if raw.Headers != nil {
		cfg.Headers = make(http.Header, len(raw.Headers))
		for key, value := range raw.Headers {
			canonicalKey := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(key))
			cfg.Headers.Set(canonicalKey, value)
		}
	} else if raw.LegacyHeaders != nil {
		cfg.Headers = make(http.Header, len(raw.LegacyHeaders))
		for key, values := range raw.LegacyHeaders {
			canonicalKey := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(key))
			for _, value := range values {
				cfg.Headers.Add(canonicalKey, value)
			}
		}
	}

	if raw.JSONBody != nil {
		bodyBytes, err := json.Marshal(raw.JSONBody)
		if err != nil {
			return cfg, fmt.Errorf("invalid json_body: %w", err)
		}
		cfg.JSONBody = string(bodyBytes)
	} else if raw.LegacyBody != "" {
		cfg.JSONBody = raw.LegacyBody
	}
	if raw.RPS != nil {
		cfg.RPS = *raw.RPS
	}
	if raw.Retries != nil {
		cfg.Retries = *raw.Retries
	}
	if raw.TotalRequests != nil {
		cfg.TotalRequests = *raw.TotalRequests
	} else if raw.LegacyRequests != nil {
		cfg.TotalRequests = *raw.LegacyRequests
	}
	if raw.Concurrency != nil {
		cfg.Concurrency = *raw.Concurrency
	}
	if raw.Output != nil {
		cfg.Output = *raw.Output
	}

	warmupValue := raw.WarmupDuration
	if warmupValue == "" {
		warmupValue = raw.LegacyWarmup
	}

	if warmupValue != "" {
		duration, err := time.ParseDuration(warmupValue)
		if err != nil {
			return cfg, fmt.Errorf("invalid warmup duration: %w", err)
		}
		cfg.WarmupDuration = duration
	}

	if raw.RequestTimeout != "" {
		duration, err := time.ParseDuration(raw.RequestTimeout)
		if err != nil {
			return cfg, fmt.Errorf("invalid timeout duration: %w", err)
		}
		cfg.RequestTimeout = duration
	}

	return cfg, nil
}

type configFile struct {
	URL            string              `json:"url" yaml:"url"`
	Method         string              `json:"method" yaml:"method"`
	Headers        map[string]string   `json:"headers" yaml:"headers"`
	LegacyHeaders  map[string][]string `json:"legacy_headers" yaml:"legacy_headers"`
	JSONBody       any                 `json:"json_body" yaml:"json_body"`
	LegacyBody     string              `json:"body" yaml:"body"`
	RPS            *int                `json:"rps" yaml:"rps"`
	Retries        *int                `json:"retries" yaml:"retries"`
	WarmupDuration string              `json:"warmup" yaml:"warmup"`
	LegacyWarmup   string              `json:"warmup_duration" yaml:"warmup_duration"`
	TotalRequests  *int                `json:"requests" yaml:"requests"`
	LegacyRequests *int                `json:"total_requests" yaml:"total_requests"`
	Concurrency    *int                `json:"concurrency" yaml:"concurrency"`
	RequestTimeout string              `json:"request_timeout" yaml:"request_timeout"`
	Output         *string             `json:"output" yaml:"output"`
}

type resultExport struct {
	Configuration exportConfig  `json:"configuration"`
	Metrics       exportMetrics `json:"metrics"`
}

type exportConfig struct {
	URL            string              `json:"url"`
	Method         string              `json:"method"`
	Headers        map[string]string   `json:"headers,omitempty"`
	JSONBody       any                 `json:"json_body,omitempty"`
	TotalRequests  int                 `json:"requests"`
	Concurrency    int                 `json:"concurrency"`
	RPS            int                 `json:"rps"`
	Retries        int                 `json:"retries"`
	WarmupDuration string              `json:"warmup"`
	RequestTimeout string              `json:"request_timeout"`
}

type exportMetrics struct {
	TotalRequests          int64                        `json:"total_requests"`
	Completed              int64                        `json:"completed_requests"`
	Successes              int64                        `json:"success_count"`
	Failures               int64                        `json:"failure_count"`
	Retries                int64                        `json:"retry_attempts"`
	AvgLatencyMS           float64                      `json:"avg_latency_ms"`
	P50LatencyMS           float64                      `json:"p50_latency_ms"`
	P90LatencyMS           float64                      `json:"p90_latency_ms"`
	P95LatencyMS           float64                      `json:"p95_latency_ms"`
	P99LatencyMS           float64                      `json:"p99_latency_ms"`
	ThroughputRPS          float64                      `json:"throughput_rps"`
	TPS                    float64                      `json:"tps"`
	SuccessRate            float64                      `json:"success_rate"`
	ErrorRate              float64                      `json:"error_rate"`
	StatusCodeDistribution map[string]int64             `json:"status_code_distribution,omitempty"`
	ErrorSummary           map[string]int64             `json:"error_summary,omitempty"`
	FailedRequestSamples   []metrics.FailedSample       `json:"failed_request_samples,omitempty"`
	Duration               string                       `json:"duration"`
}

func exportResults(cfg config, snapshot metrics.Snapshot) error {
	report := resultExport{
		Configuration: exportConfig{
			URL:            cfg.URL,
			Method:         cfg.Method,
			Headers:        flattenHeaders(cfg.Headers),
			JSONBody:       parseJSONBody(cfg.JSONBody),
			TotalRequests:  cfg.TotalRequests,
			Concurrency:    cfg.Concurrency,
			RPS:            cfg.RPS,
			Retries:        cfg.Retries,
			WarmupDuration: cfg.WarmupDuration.String(),
			RequestTimeout: cfg.RequestTimeout.String(),
		},
		Metrics: exportMetrics{
			TotalRequests:          snapshot.TotalRequests,
			Completed:              snapshot.Completed,
			Successes:              snapshot.Successes,
			Failures:               snapshot.Failures,
			Retries:                snapshot.Retries,
			AvgLatencyMS:           snapshot.AverageLatencyMilliseconds(),
			P50LatencyMS:           snapshot.PercentileMilliseconds(50),
			P90LatencyMS:           snapshot.PercentileMilliseconds(90),
			P95LatencyMS:           snapshot.PercentileMilliseconds(95),
			P99LatencyMS:           snapshot.PercentileMilliseconds(99),
			ThroughputRPS:          snapshot.Throughput(),
			TPS:                    snapshot.Throughput(),
			SuccessRate:            snapshot.SuccessRate(),
			ErrorRate:              snapshot.ErrorRate(),
			StatusCodeDistribution: stringifyStatusCodes(snapshot.StatusCodes),
			ErrorSummary:           snapshot.ErrorSummary,
			FailedRequestSamples:   snapshot.FailedSamples,
			Duration:               snapshot.Duration.String(),
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Clean(cfg.Output)
	return os.WriteFile(outputPath, data, 0o644)
}

func stringifyStatusCodes(values map[int]int64) map[string]int64 {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]int64, len(values))
	for code, count := range values {
		result[fmt.Sprintf("%d", code)] = count
	}

	return result
}

func emitCLIError(message, details string) {
	payload := map[string]string{
		"status": "failed",
		"error":  message,
	}

	if details != "" {
		payload["details"] = details
	}

	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, message+":", details)
		return
	}

	fmt.Fprintln(os.Stderr, string(data))
}

func flattenHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}

	return result
}

func parseJSONBody(body string) any {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return body
	}

	return parsed
}

func normalizeMethod(method string) string {
	switch method {
	case "", http.MethodGet:
		return http.MethodGet
	case http.MethodPost:
		return http.MethodPost
	default:
		return strings.ToUpper(method)
	}
}

func newTransport(concurrency int) *http.Transport {
	idlePoolSize := max(concurrency*4, 512)

	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          idlePoolSize,
		MaxIdleConnsPerHost:   idlePoolSize,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

type headerFlag struct {
	values []string
	set    bool
}

func newHeaderFlag(headers http.Header) *headerFlag {
	values := make([]string, 0, len(headers))
	for key, headerValues := range headers {
		for _, value := range headerValues {
			values = append(values, fmt.Sprintf("%s: %s", key, value))
		}
	}

	return &headerFlag{values: values}
}

func (h *headerFlag) String() string {
	return strings.Join(h.values, ", ")
}

func (h *headerFlag) Set(value string) error {
	if !h.set {
		h.values = h.values[:0]
		h.set = true
	}
	h.values = append(h.values, value)
	return nil
}

func (h *headerFlag) Header() (http.Header, error) {
	headers := make(http.Header, len(h.values))

	for _, raw := range h.values {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header %q: expected 'Key: Value' format", raw)
		}

		key := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid header %q: key and value are required", raw)
		}

		headers.Add(key, value)
	}

	return headers, nil
}
