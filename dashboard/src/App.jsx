import { useState } from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from 'recharts';

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '');

const initialForm = {
  url: 'https://example.com',
  method: 'GET',
  requests: 1000,
  concurrency: 50,
  duration: 0,
  rampUp: 0,
  requestTimeout: 5,
  rps: 0,
  retries: 0,
  body: '',
  headers: '',
  warmup: '5s'
};

const accentByKey = {
  p50: '#38bdf8',
  p90: '#60a5fa',
  p95: '#fbbf24',
  p99: '#fb7185'
};

function App() {
  const [form, setForm] = useState(initialForm);
  const [isRunning, setIsRunning] = useState(false);
  const [error, setError] = useState('');
  const [errorDetails, setErrorDetails] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const [runResult, setRunResult] = useState(null);

  const handleChange = (event) => {
    const { name, value } = event.target;
    setForm((current) => ({
      ...current,
      [name]: ['requests', 'concurrency', 'duration', 'rampUp', 'requestTimeout', 'rps', 'retries'].includes(name) ? Number(value) : value
    }));
  };

  const handleSubmit = async (event) => {
    event.preventDefault();
    setIsRunning(true);
    setError('');
    setErrorDetails('');
    setSuccessMessage('');

    const parsedHeaders = parseHeaders(form.headers, form.method);
    if (parsedHeaders.error) {
      setError(parsedHeaders.error);
      setIsRunning(false);
      return;
    }

    const trimmedBody = form.body.trim();
    let parsedJsonBody;
    if (trimmedBody) {
      try {
        parsedJsonBody = JSON.parse(trimmedBody);
      } catch {
        setError('JSON body must be valid JSON before starting the test.');
        setIsRunning(false);
        return;
      }
    }

    if (form.duration > 0 && form.rampUp > form.duration) {
      setError('Ramp-up must be less than or equal to duration.');
      setIsRunning(false);
      return;
    }

    if (form.requestTimeout <= 0) {
      setError('Request timeout must be greater than zero.');
      setIsRunning(false);
      return;
    }

    try {
      const response = await fetch(`${apiBaseUrl}/run-test`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          url: form.url,
          method: form.method,
          headers: parsedHeaders.value ?? {},
          json_body: parsedJsonBody,
          requests: form.requests,
          concurrency: form.concurrency,
          duration: form.duration,
          rampUp: form.rampUp,
          request_timeout: form.requestTimeout,
          rps: form.rps,
          retries: form.retries,
          warmup: form.warmup
        })
      });

      const payload = await response.json();
      if (!response.ok) {
        throw new Error(formatApiError(payload));
      }

      setRunResult(payload);
      setSuccessMessage('Load test completed successfully. Metrics have been refreshed.');
    } catch (submitError) {
      const { message, details } = parseUiError(submitError);
      setError(message);
      setErrorDetails(details);
    } finally {
      setIsRunning(false);
    }
  };

  const metrics = runResult?.results?.metrics ?? null;
  const chartData = metrics
    ? [
        { key: 'p50', label: 'P50', value: metrics.p50_latency_ms },
        { key: 'p90', label: 'P90', value: metrics.p90_latency_ms },
        { key: 'p95', label: 'P95', value: metrics.p95_latency_ms },
        { key: 'p99', label: 'P99', value: metrics.p99_latency_ms }
      ]
    : [];
  const statusCodeData = metrics?.status_code_distribution
    ? Object.entries(metrics.status_code_distribution).map(([code, count]) => ({
        code,
        count
      }))
    : [];
  const failedSamples = runResult?.failedRequestSamples ?? metrics?.failed_request_samples ?? [];
  const errorSummary = runResult?.errorSummary ?? metrics?.error_summary ?? {};

  return (
    <div className="relative overflow-hidden">
      <div className="absolute inset-0 bg-grid bg-[size:42px_42px] opacity-30" />
      <div className="relative mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-8">
        <header className="grid gap-6 rounded-[2rem] border border-white/10 bg-white/8 p-6 shadow-panel backdrop-blur xl:grid-cols-[1.4fr_0.9fr]">
          <div className="space-y-5">
            <div className="inline-flex items-center rounded-full border border-aqua/30 bg-aqua/10 px-3 py-1 text-sm font-semibold text-aqua">
              Performance Control Center
            </div>
            <div className="space-y-3">
              <h1 className="max-w-2xl text-4xl font-extrabold tracking-tight text-white sm:text-5xl">
                Launch and analyze load tests from one responsive dashboard.
              </h1>
              <p className="max-w-2xl text-base leading-7 text-slate-300 sm:text-lg">
                Configure traffic patterns, trigger the Go engine, and review latency and throughput
                signals in a clean operator-friendly workspace.
              </p>
            </div>
            <div className="inline-flex w-fit items-center rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-slate-300">
              API {apiBaseUrl || 'same-origin'}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <HeroStat label="Default Flow" value="GET / POST" tone="aqua" />
            <HeroStat label="Readout" value="JSON + Charts" tone="sun" />
            <HeroStat label="Health" value={metrics ? `${metrics.success_count} OK` : 'Awaiting run'} tone="mint" />
            <HeroStat
              label="Throughput"
              value={metrics ? `${metrics.throughput_rps.toFixed(2)} rps` : 'No data'}
              tone="coral"
            />
          </div>
        </header>

        <main className="grid gap-8 xl:grid-cols-[420px_minmax(0,1fr)]">
          <section className="rounded-[2rem] border border-white/10 bg-slate-900/70 p-6 shadow-panel backdrop-blur">
            <div className="mb-6 flex items-center justify-between">
              <div>
                <h2 className="text-2xl font-bold text-white">Test Configuration</h2>
                <p className="mt-1 text-sm text-slate-400">Tune the request profile and launch the backend runner.</p>
              </div>
              <div className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-slate-300">
                Live form
              </div>
            </div>

            <form className="space-y-5" onSubmit={handleSubmit}>
              <FormField label="Target URL">
                <input
                  className={inputClass}
                  name="url"
                  value={form.url}
                  onChange={handleChange}
                  placeholder="https://api.example.com/health"
                  required
                />
              </FormField>

              <div className="grid gap-5 sm:grid-cols-2">
                <FormField label="Method">
                  <select className={inputClass} name="method" value={form.method} onChange={handleChange}>
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                  </select>
                </FormField>
                <FormField label="Requests">
                  <input
                    className={inputClass}
                    type="number"
                    min="1"
                    name="requests"
                    value={form.requests}
                    onChange={handleChange}
                    required
                  />
                </FormField>
              </div>

              <div className="grid gap-5 sm:grid-cols-2">
                <FormField label="Concurrency">
                  <input
                    className={inputClass}
                    type="number"
                    min="1"
                    name="concurrency"
                    value={form.concurrency}
                    onChange={handleChange}
                    required
                  />
                </FormField>
                <FormField label="RPS">
                  <input
                    className={inputClass}
                    type="number"
                    min="0"
                    name="rps"
                    value={form.rps}
                    onChange={handleChange}
                  />
                </FormField>
              </div>

              <div className="grid gap-5 sm:grid-cols-2">
                <FormField label="Duration (Seconds)">
                  <input
                    className={inputClass}
                    type="number"
                    min="0"
                    name="duration"
                    value={form.duration}
                    onChange={handleChange}
                  />
                </FormField>
                <FormField label="Ramp-Up (Seconds)">
                  <input
                    className={inputClass}
                    type="number"
                    min="0"
                    name="rampUp"
                    value={form.rampUp}
                    onChange={handleChange}
                  />
                </FormField>
              </div>

              <FormField label="Request Timeout (Seconds)">
                <input
                  className={inputClass}
                  type="number"
                  min="1"
                  name="requestTimeout"
                  value={form.requestTimeout}
                  onChange={handleChange}
                />
              </FormField>

              <FormField label="Retries">
                <input
                  className={inputClass}
                  type="number"
                  min="0"
                  name="retries"
                  value={form.retries}
                  onChange={handleChange}
                />
              </FormField>

              <FormField label="Headers (JSON)">
                <textarea
                  className={`${inputClass} min-h-32 resize-y`}
                  name="headers"
                  value={form.headers}
                  onChange={handleChange}
                  placeholder={`{\n  "Authorization": "Bearer YOUR_TOKEN",\n  "Content-Type": "application/json",\n  "x-org-id": "bm",\n  "x-site-id": "bm_dev_001"\n}`}
                />
              </FormField>

              <FormField label="JSON Body">
                <textarea
                  className={`${inputClass} min-h-40 resize-y font-mono text-sm`}
                  name="body"
                  value={form.body}
                  onChange={handleChange}
                  placeholder={`{\n  "cartId": "CART-123",\n  "itemId": "ITEM-456",\n  "quantity": 2\n}`}
                />
              </FormField>

              <button
                className="group inline-flex w-full items-center justify-center rounded-2xl bg-gradient-to-r from-aqua via-sky-400 to-sun px-5 py-4 text-base font-bold text-slate-950 transition hover:scale-[1.01] disabled:cursor-not-allowed disabled:opacity-60"
                type="submit"
                disabled={isRunning}
              >
                {isRunning ? 'Running test...' : 'Start Test'}
              </button>

              {isRunning && form.duration > 0 ? (
                <div className="rounded-2xl border border-aqua/20 bg-aqua/10 px-4 py-3 text-sm font-medium text-cyan-100">
                  Running for {form.duration} seconds
                  {form.rampUp > 0 ? ` with ${form.rampUp}s ramp-up` : ''}
                </div>
              ) : null}

              {error ? (
                <div className="rounded-2xl border border-coral/30 bg-coral/10 px-4 py-3 text-sm font-medium text-rose-100 shadow-inner shadow-coral/10">
                  <div>{error}</div>
                  {errorDetails ? (
                    <pre className="mt-3 overflow-x-auto whitespace-pre-wrap break-words rounded-xl border border-white/10 bg-slate-950/30 p-3 text-xs font-normal leading-6 text-rose-100">
                      {errorDetails}
                    </pre>
                  ) : null}
                </div>
              ) : null}

              {successMessage ? (
                <div className="rounded-2xl border border-mint/30 bg-mint/10 px-4 py-3 text-sm font-medium text-emerald-100 shadow-inner shadow-mint/10">
                  {successMessage}
                </div>
              ) : null}
            </form>
          </section>

          <section className="space-y-8">
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <MetricCard
                label="TPS"
                value={metrics ? `${metrics.tps.toFixed(2)} req/s` : '--'}
                accent="bg-cyan-400"
                description="Transactions processed per second"
              />
              <MetricCard
                label="Successful Requests"
                value={metrics ? formatInteger(metrics.success_count) : '--'}
                accent="bg-emerald-400"
                description="Requests completed without error"
              />
              <MetricCard
                label="Failed Requests"
                value={metrics ? formatInteger(metrics.failure_count) : '--'}
                accent="bg-rose-400"
                description="Requests that exhausted retries or returned failure"
              />
              <MetricCard
                label="Success Rate"
                value={metrics ? `${metrics.success_rate.toFixed(2)}%` : '--'}
                accent="bg-teal-400"
                description="Share of completed requests that succeeded"
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <MetricCard
                label="Error Rate"
                value={metrics ? `${metrics.error_rate.toFixed(2)}%` : '--'}
                accent="bg-rose-300"
                description="Share of completed requests that failed"
              />
              <MetricCard
                label="Average Latency"
                value={metrics ? `${metrics.avg_latency_ms.toFixed(2)} ms` : '--'}
                accent="bg-amber-300"
                description="End-to-end per request average"
              />
              <MetricCard
                label="Throughput"
                value={metrics ? `${metrics.throughput_rps.toFixed(2)} rps` : '--'}
                accent="bg-sky-400"
                description="Measured completion rate"
              />
              <MetricCard
                label="Retries"
                value={metrics ? formatInteger(metrics.retry_attempts) : '--'}
                accent="bg-violet-300"
                description="Retry attempts triggered by failed requests"
              />
            </div>

            <div className="grid gap-8 2xl:grid-cols-[1.2fr_0.8fr]">
              <article className="rounded-[2rem] border border-white/10 bg-slate-900/70 p-6 shadow-panel backdrop-blur">
                <div className="mb-6 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <h2 className="text-2xl font-bold text-white">Latency Profile</h2>
                    <p className="mt-1 text-sm text-slate-400">
                      Percentile distribution from the most recent completed run.
                    </p>
                  </div>
                  <div className="text-sm text-slate-400">
                    {metrics ? `Duration ${metrics.duration}` : 'Run a test to populate the chart'}
                  </div>
                </div>

                <div className="h-80 w-full">
                  {chartData.length > 0 ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                        <CartesianGrid stroke="rgba(148, 163, 184, 0.16)" vertical={false} />
                        <XAxis dataKey="label" tick={{ fill: '#cbd5e1', fontSize: 12 }} axisLine={false} tickLine={false} />
                        <YAxis
                          tick={{ fill: '#cbd5e1', fontSize: 12 }}
                          axisLine={false}
                          tickLine={false}
                          unit="ms"
                        />
                        <Tooltip
                          cursor={{ fill: 'rgba(255,255,255,0.04)' }}
                          contentStyle={{
                            backgroundColor: '#0f172a',
                            border: '1px solid rgba(148,163,184,0.18)',
                            borderRadius: '16px',
                            color: '#e2e8f0'
                          }}
                          formatter={(value) => [`${Number(value).toFixed(2)} ms`, 'Latency']}
                        />
                        <Bar dataKey="value" radius={[14, 14, 0, 0]}>
                          {chartData.map((entry) => (
                            <Cell key={entry.key} fill={accentByKey[entry.key]} />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  ) : (
                    <EmptyState />
                  )}
                </div>
              </article>

              <article className="rounded-[2rem] border border-white/10 bg-slate-900/70 p-6 shadow-panel backdrop-blur">
                <h2 className="text-2xl font-bold text-white">Percentile Snapshot</h2>
                <p className="mt-1 text-sm text-slate-400">Fast view of your tail latency spread.</p>
                <div className="mt-6 space-y-4">
                  <PercentileRow label="P50" value={metrics?.p50_latency_ms} tone="bg-sky-400" />
                  <PercentileRow label="P95" value={metrics?.p95_latency_ms} tone="bg-amber-300" />
                  <PercentileRow label="P99" value={metrics?.p99_latency_ms} tone="bg-rose-400" />
                </div>

                <div className="mt-8 rounded-3xl border border-white/10 bg-white/5 p-5">
                  <h3 className="text-sm font-semibold uppercase tracking-[0.24em] text-slate-400">Latest Run</h3>
                  <dl className="mt-4 space-y-3">
                    <DetailRow label="URL" value={runResult?.results?.configuration?.url ?? 'Not started'} />
                    <DetailRow label="Method" value={runResult?.results?.configuration?.method ?? '--'} />
                    <DetailRow
                      label="Requests"
                      value={
                        runResult?.results?.configuration?.requests
                          ? formatInteger(runResult.results.configuration.requests)
                          : '--'
                      }
                    />
                    <DetailRow
                      label="Concurrency"
                      value={
                        runResult?.results?.configuration?.concurrency
                          ? formatInteger(runResult.results.configuration.concurrency)
                          : '--'
                      }
                    />
                    <DetailRow
                      label="Retries"
                      value={
                        metrics?.retry_attempts !== undefined ? formatInteger(metrics.retry_attempts) : '--'
                      }
                    />
                  </dl>
                </div>
              </article>
            </div>

            <div className="grid gap-8 xl:grid-cols-[0.95fr_1.05fr]">
              <article className="rounded-[2rem] border border-white/10 bg-slate-900/70 p-6 shadow-panel backdrop-blur">
                <div className="mb-6 flex items-center justify-between">
                  <div>
                    <h2 className="text-2xl font-bold text-white">Status Code Distribution</h2>
                    <p className="mt-1 text-sm text-slate-400">Response mix across the latest test run.</p>
                  </div>
                </div>

                {statusCodeData.length > 0 ? (
                  <div className="space-y-3">
                    {statusCodeData.map((entry) => (
                      <div
                        key={entry.code}
                        className="flex items-center justify-between rounded-2xl border border-white/10 bg-white/5 px-4 py-3"
                      >
                        <span className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-300">
                          {entry.code}
                        </span>
                        <span className="text-lg font-bold text-white">{formatInteger(entry.count)}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <EmptyState message="Run a test to see response status code distribution." />
                )}

                {Object.keys(errorSummary).length > 0 ? (
                  <div className="mt-8 rounded-3xl border border-white/10 bg-white/5 p-5">
                    <h3 className="text-sm font-semibold uppercase tracking-[0.24em] text-slate-400">Error Summary</h3>
                    <div className="mt-4 space-y-3">
                      {Object.entries(errorSummary).map(([message, count]) => (
                        <div key={message} className="flex items-start justify-between gap-4">
                          <span className="text-sm text-slate-300">{message}</span>
                          <span className="text-sm font-semibold text-white">{formatInteger(count)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </article>

              <article className="rounded-[2rem] border border-white/10 bg-slate-900/70 p-6 shadow-panel backdrop-blur">
                <div className="mb-6 flex items-center justify-between">
                  <div>
                    <h2 className="text-2xl font-bold text-white">Failed Requests</h2>
                    <p className="mt-1 text-sm text-slate-400">Sample failures captured for fast debugging.</p>
                  </div>
                </div>

                {failedSamples.length > 0 ? (
                  <div className="overflow-hidden rounded-3xl border border-white/10">
                    <div className="grid grid-cols-[140px_minmax(0,1fr)] bg-white/5 px-4 py-3 text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">
                      <div>Status</div>
                      <div>Error</div>
                    </div>
                    <div className="divide-y divide-white/10">
                      {failedSamples.map((sample, index) => (
                        <div
                          key={`${sample.status_code}-${sample.error || 'none'}-${index}`}
                          className="grid grid-cols-[140px_minmax(0,1fr)] px-4 py-3 text-sm"
                        >
                          <div className="font-semibold text-rose-200">
                            {sample.status_code || 'ERR'}
                          </div>
                          <div className="break-words text-slate-300">{sample.error || 'Request failed'}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <EmptyState message="No failed request samples captured yet." />
                )}
              </article>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

function HeroStat({ label, value, tone }) {
  const tones = {
    aqua: 'from-aqua/20 to-sky-400/5 text-aqua',
    sun: 'from-sun/20 to-amber-300/5 text-sun',
    mint: 'from-mint/20 to-emerald-300/5 text-mint',
    coral: 'from-coral/20 to-rose-300/5 text-coral'
  };

  return (
    <div className={`rounded-[1.5rem] border border-white/10 bg-gradient-to-br ${tones[tone]} p-4`}>
      <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-300">{label}</div>
      <div className="mt-5 text-2xl font-extrabold text-white">{value}</div>
    </div>
  );
}

function MetricCard({ label, value, accent, description }) {
  return (
    <article className="rounded-[1.75rem] border border-white/10 bg-slate-900/70 p-5 shadow-panel backdrop-blur">
      <div className="flex items-center gap-3">
        <span className={`h-3 w-3 rounded-full ${accent}`} />
        <p className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-400">{label}</p>
      </div>
      <p className="mt-5 text-3xl font-extrabold text-white">{value}</p>
      <p className="mt-2 text-sm leading-6 text-slate-400">{description}</p>
    </article>
  );
}

function FormField({ label, children }) {
  return (
    <label className="block">
      <span className="mb-2 block text-sm font-semibold uppercase tracking-[0.2em] text-slate-400">{label}</span>
      {children}
    </label>
  );
}

function PercentileRow({ label, value, tone }) {
  const display = value !== undefined && value !== null ? `${value.toFixed(2)} ms` : '--';
  return (
    <div className="rounded-3xl border border-white/10 bg-white/5 p-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className={`h-3 w-3 rounded-full ${tone}`} />
          <span className="text-sm font-semibold uppercase tracking-[0.22em] text-slate-300">{label}</span>
        </div>
        <span className="text-lg font-bold text-white">{display}</span>
      </div>
    </div>
  );
}

function DetailRow({ label, value }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-sm text-slate-400">{label}</dt>
      <dd className="max-w-[60%] text-right text-sm font-semibold text-slate-200">{value}</dd>
    </div>
  );
}

function EmptyState({ message = 'Start a test to render percentile bars and latency trends here.' }) {
  return (
    <div className="flex h-full items-center justify-center rounded-[1.75rem] border border-dashed border-white/10 bg-white/5 px-6 text-center text-sm text-slate-400">
      {message}
    </div>
  );
}

function formatInteger(value) {
  return new Intl.NumberFormat().format(value);
}

function parseHeaders(rawHeaders, method) {
  const trimmed = rawHeaders.trim();
  if (!trimmed) {
    return { value: {} };
  }

  let parsed;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { error: 'Invalid JSON in headers.' };
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { error: 'Headers must be a JSON object.' };
  }

  for (const [key, value] of Object.entries(parsed)) {
    if (typeof value !== 'string') {
      return { error: `Header "${key}" must have a string value.` };
    }
  }

  if (
    method === 'POST' &&
    typeof parsed.Authorization === 'string' &&
    parsed.Authorization.trim() === ''
  ) {
    return { error: 'Authorization header cannot be empty when provided.' };
  }

  return { value: parsed };
}

function formatApiError(payload) {
  const message = payload?.error || 'Failed to run load test';
  const details = formatErrorDetails(payload?.details);

  if (!details) {
    return message;
  }

  return `${message}\n${details}`;
}

function parseUiError(error) {
  const raw = error?.message || 'Failed to run load test';
  const [message, ...rest] = raw.split('\n');

  return {
    message,
    details: rest.join('\n').trim()
  };
}

function formatErrorDetails(details) {
  if (!details) {
    return '';
  }

  if (typeof details === 'string') {
    return details;
  }

  if (Array.isArray(details)) {
    return details.map((entry) => formatErrorDetails(entry)).filter(Boolean).join('\n');
  }

  if (typeof details === 'object') {
    return Object.entries(details)
      .filter(([, value]) => value !== undefined && value !== null && value !== '')
      .map(([key, value]) => `${key}: ${typeof value === 'string' ? value : JSON.stringify(value)}`)
      .join('\n');
  }

  return String(details);
}

const inputClass =
  'w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-base text-white outline-none transition placeholder:text-slate-500 focus:border-aqua/50 focus:bg-white/10 focus:ring-2 focus:ring-aqua/20';

export default App;
