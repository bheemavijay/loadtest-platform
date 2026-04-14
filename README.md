🚀 Distributed Load Testing Platform

A high-performance load testing system built using Go, Node.js, and
React to simulate high-concurrency API traffic.

------------------------------------------------------------------------

🧠 Architecture

React UI → Node.js Backend → Go Load Engine → Metrics → UI

-   React: UI for configuring tests and viewing results
-   Node.js: Orchestrates execution and manages processes
-   Go: High-performance engine using goroutines

------------------------------------------------------------------------

⚙️ Features

-   Concurrency-based load testing using goroutines
-   Accurate RPS control using token bucket rate limiter
-   Time-based execution using context deadlines
-   Latency tracking (min, max, avg, P95, P99)
-   Lock-free metrics using atomic counters
-   Memory-efficient rolling latency buffer
-   Fast payload generation using math/rand

------------------------------------------------------------------------

⚡ Performance Optimizations

-   Replaced ticker with token bucket rate limiter
-   Removed global mutex using atomic counters
-   Avoided full latency storage to reduce memory usage
-   Replaced crypto/rand with math/rand
-   Limited concurrent executions in backend

------------------------------------------------------------------------

▶️ Run Complete Application

Step 1: Start Backend

cd backend

npm install

npm start

------------------------------------------------------------------------

Step 2: Start Frontend

cd dashboard

npm install

npm run dev

------------------------------------------------------------------------

Go Engine

The Go load engine is executed automatically by the backend. You do NOT
need to run it manually.

------------------------------------------------------------------------

Open in browser: http://localhost:5173

------------------------------------------------------------------------

🧪 How to Test

Basic Test (GET)

URL: https://jsonplaceholder.typicode.com/posts

Method:GET

Requests: 50

Concurrency: 5

------------------------------------------------------------------------

Throughput Test

URL: https://jsonplaceholder.typicode.com/posts

Method:GET

RPS: 100

Duration: 10

------------------------------------------------------------------------

POST Test

URL: https://jsonplaceholder.typicode.com/posts

Method: POST

{

    “title”: “test”,

    “body”: “load test”,

    “userId”: 1

}

------------------------------------------------------------------------

📊 Load Modes

Fixed Mode → use requests
Throughput Mode → use rps + duration

Do not use both together.

------------------------------------------------------------------------

💡 Why Go?

Go provides lightweight goroutines (~2KB each), making it ideal for
high-concurrency workloads.

------------------------------------------------------------------------

⚠️ Current Limitations

-   Single-node system (not distributed yet)
-   No persistence of results
-   Long-running tests are synchronous
-   Metrics may vary under high CPU load

------------------------------------------------------------------------

🚀 Future Improvements

-   Distributed worker nodes
-   Real-time metrics (WebSockets)
-   Persistent storage
-   Queue-based execution

------------------------------------------------------------------------

📌 Author

Vijay Krishnan
