# Load Test Dashboard

React + Tailwind dashboard for running load tests through the Express backend.

## Setup

```bash
npm install
```

## Run

```bash
npm run dev
```

The Vite dev server proxies `POST /run-test` to `http://localhost:3000`, so the Express backend should be running locally.

## Features

- Responsive dashboard layout
- Load test form for URL, method, requests, concurrency, RPS, and retries
- Summary cards for success, failure, throughput, and average latency
- Percentile chart and latency snapshot cards
- Clean API integration with the Express backend
