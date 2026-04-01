# Load Test Backend

Express API for executing the Go load testing engine and returning JSON results.

## Setup

```bash
npm install
```

## Run

```bash
npm start
```

## Environment

- `PORT`: HTTP port, default `3000`
- `GO_BINARY_PATH`: optional path to a compiled Go binary
- `LOADTEST_EXECUTION_TIMEOUT_MS`: execution timeout in milliseconds, default `600000`

## Endpoint

`POST /run-test`

Example request body:

```json
{
  "url": "https://example.com/api/orders",
  "method": "POST",
  "headers": {
    "Authorization": ["Bearer token"],
    "X-Test-Run": ["nightly"]
  },
  "json_body": "{\"region\":\"us-east-1\",\"size\":100}",
  "total_requests": 1000,
  "concurrency": 50,
  "rps": 200,
  "retries": 2,
  "warmup_duration": "5s",
  "request_timeout": "10s"
}
```

The API writes a temporary config file, runs the Go tool with `-config`, reads the generated JSON results file, and returns the parsed results in the response.

By default the backend executes:

```bash
go run ./cmd/loadtest -config <temp-config>
```

Set `GO_BINARY_PATH` if you want to execute a compiled binary instead.
