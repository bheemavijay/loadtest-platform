# 🚀 Distributed Load Testing Platform

A high-performance load testing system built using Go, Node.js, and React to simulate high-concurrency API traffic.

---

## 🧠 Architecture

React UI → Node.js Backend → Go Load Engine → Metrics → UI

- React: User interface for configuring tests and viewing results
- Node.js: Orchestrates test execution and manages processes
- Go: High-performance engine for generating concurrent requests

---

## ⚙️ Features

- 🔥 Concurrency-based load testing using goroutines
- ⚡ Accurate RPS control using token bucket rate limiter
- ⏱️ Time-based execution using context deadlines
- 📊 Latency tracking (min, max, avg, P95, P99)
- 🚀 Optimized metrics using atomic counters (lock-free)
- 🧠 Rolling latency buffer (low memory usage)
- ⚡ Fast payload generation using math/rand

---

## ⚡ Performance Optimizations

- Replaced ticker with token bucket rate limiter
- Removed global mutex using atomic counters
- Optimized memory by avoiding full latency storage
- Replaced crypto/rand with math/rand for faster generation
- Limited concurrent executions in backend to prevent overload

---

## ▶️ Run Complete Application

### Step 1: Start Backend (Node.js)
cd backend

npm install

npm start

---

### Terminal 2 – Frontend
cd dashboard
npm install
npm run dev

---

Then open:
http://localhost:5173

---

### 📊 Sample Test Config
{
  "url": "https://jsonplaceholder.typicode.com/posts",

  "method": "GET",

  "rps": 100,

  "duration": 30,

  "concurrency": 10,

  "request_timeout": 5
}

---

## 💡 Why Go?

Go provides lightweight goroutines, making it ideal for simulating high-concurrency workloads efficiently with low memory overhead.

---

## 🚀 Future Improvements

- Distributed load execution across multiple nodes
- Real-time metrics streaming (WebSockets)
- Result storage and history

---

## 📌 Author

Vijay Krishnan

---