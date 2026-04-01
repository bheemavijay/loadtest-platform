# 🚀 Load Testing Platform

A full-stack load testing platform built with:

- ⚡ Go (high-performance load engine)
- 🌐 Node.js (orchestrator API)
- 🎨 React + Tailwind (dashboard UI)

## Features

- Dynamic payload generation
- RPS & concurrency control
- Retry support
- Warm-up phase
- JSON/YAML config support
- Real-time metrics:
  - TPS
  - Success/Error rate
  - Latency percentiles (P50, P95, P99)
- Error tracking + failed request samples

## Architecture

UI (React) → Backend (Node) → Load Engine (Go)

## Setup

### Backend
```bash
cd backend
npm install
npm start

# 🚀 Load Testing Platform

A full-stack load testing platform built with:

- ⚡ Go (high-performance load engine)
- 🌐 Node.js (orchestrator API)
- 🎨 React + Tailwind (dashboard UI)

## Features

- Dynamic payload generation
- RPS & concurrency control
- Retry support
- Warm-up phase
- JSON/YAML config support
- Real-time metrics:
  - TPS
  - Success/Error rate
  - Latency percentiles (P50, P95, P99)
- Error tracking + failed request samples

## Architecture

UI (React) → Backend (Node) → Load Engine (Go)

## Setup

### Backend
```bash
cd backend
npm install
npm start

### Frontend
```bash
cd dashboard
npm install
npm run dev

### Go Engine
```bash
go run ./cmd/loadtest

### Author

Vijay K