'use strict';

const cors = require('cors');
const express = require('express');
const fs = require('fs/promises');
const os = require('os');
const path = require('path');
const { randomUUID } = require('crypto');
const { spawn } = require('child_process');

const PORT = Number.parseInt(process.env.PORT || '3000', 10);
const GO_BINARY_PATH = process.env.GO_BINARY_PATH || path.resolve(__dirname, '..', '..', 'loadtest');
const EXECUTION_TIMEOUT_MS = Number.parseInt(process.env.LOADTEST_EXECUTION_TIMEOUT_MS || '600000', 10);
const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const NODE_ENV = process.env.NODE_ENV || 'development';
const MAX_CONCURRENT_TESTS = 2;
let activeLoadTests = 0;
const corsOptions = {
  origin: ['https://loadtest-platform.vercel.app'],
  methods: ['GET', 'POST', 'PUT', 'DELETE'],
  allowedHeaders: ['Content-Type', 'Authorization']
};

const app = express();
app.disable('x-powered-by');
app.set('trust proxy', 1);
app.use(cors(corsOptions));
app.options('*', cors(corsOptions));
app.use(express.json({ limit: '1mb' }));

app.get('/healthz', (_req, res) => {
  res.status(200).json({
    status: 'ok',
    environment: NODE_ENV
  });
});

app.post('/run-test', async (req, res) => {
  if (activeLoadTests >= MAX_CONCURRENT_TESTS) {
    res.status(429).json({
      status: 'failed',
      error: 'Too many load tests are already running',
      details: `Only ${MAX_CONCURRENT_TESTS} concurrent load tests are allowed`
    });
    return;
  }

  activeLoadTests++;

  try {
    const config = normalizeConfig(req.body);
    console.log('Final load test config:', JSON.stringify(config, null, 2));
    const result = await runLoadTest(config);
    res.status(200).json({
      status: 'success',
      ...result
    });
  } catch (error) {
    const statusCode = error.statusCode || 500;
    res.status(statusCode).json({
      status: 'failed',
      error: error.publicMessage || 'Failed to run load test',
      details: error.details || undefined
    });
  } finally {
      activeLoadTests = Math.max(0, activeLoadTests - 1);
      console.log(`Active load tests: ${activeLoadTests}`);
    }
});

app.use((err, _req, res, _next) => {
  res.status(400).json({
    status: 'failed',
    error: 'Invalid JSON payload',
    details: err.message
  });
});

app.listen(PORT, () => {
  console.log(`Load test API listening on port ${PORT}`);
  console.log(`Runtime environment: ${NODE_ENV}`);
  console.log(`Load test command: ${describeCommand()}`);
});

async function runLoadTest(config) {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'loadtest-api-'));
  const configPath = path.join(tempDir, 'config.json');
  const outputPath = path.join(tempDir, 'results.json');
  const payload = { ...config, output: outputPath };

  try {
    await fs.writeFile(configPath, JSON.stringify(payload, null, 2), 'utf8');

    const execution = await executeLoadTest(configPath, PROJECT_ROOT);

    const rawResults = await fs.readFile(outputPath, 'utf8');
    const results = JSON.parse(rawResults);
    const metrics = results.metrics || {};

    return {
      requestId: randomUUID(),
      stdout: execution.stdout.trim(),
      stderr: execution.stderr.trim() || undefined,
      metrics,
      errorSummary: metrics.error_summary || {},
      failedRequestSamples: metrics.failed_request_samples || [],
      results
    };
  } catch (error) {
    throw mapExecutionError(error);
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}

function executeLoadTest(configPath, cwd) {
  const command = GO_BINARY_PATH;
  const args = ['-config', configPath];

  console.log('Executing load test command:', command, args.join(' '));

  return executeCommand(command, args, { cwd });
}

function executeCommand(command, args, options) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let stdout = '';
    let stderr = '';
    let settled = false;

    const timeout = setTimeout(() => {
      if (settled) {
        return;
      }

      settled = true;
      child.kill('SIGTERM');
      reject(createError(504, 'Load test timed out', { stderr: stderr.trim() || undefined }));
    }, EXECUTION_TIMEOUT_MS);

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('error', (error) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(timeout);
      reject(createError(500, 'Unable to start Go load test process', {
        cause: error.message,
        command
      }));
    });

    child.on('close', (code, signal) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(timeout);

      if (code === 0) {
        resolve({ stdout, stderr });
        return;
      }

      reject(createError(500, 'Go load test process failed', {
        command,
        args,
        exitCode: code,
        signal: signal || undefined,
        stdout: stdout.trim() || undefined,
        stderr: stderr.trim() || undefined
      }));
    });
  });
}

function normalizeConfig(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw createError(400, 'Request body must be a JSON object');
  }

  const config = { ...payload };
  const finalHeaders =
    config.headers && typeof config.headers === 'object' && !Array.isArray(config.headers)
      ? config.headers
      : {};

  if (config.json_body === undefined && config.body !== undefined) {
    config.json_body = config.body;
  }

  if (config.requests === undefined && config.total_requests !== undefined) {
    config.requests = config.total_requests;
  }

  if (config.warmup === undefined && config.warmup_duration !== undefined) {
    config.warmup = config.warmup_duration;
  }

  if (config.rampUp === undefined && config.ramp_up !== undefined) {
    config.rampUp = config.ramp_up;
  }

  if (typeof config.url !== 'string' || config.url.trim() === '') {
    throw createError(400, 'Config field "url" is required');
  }

  if (config.method !== undefined) {
    if (typeof config.method !== 'string') {
      throw createError(400, 'Config field "method" must be a string');
    }

    config.method = config.method.toUpperCase();
    if (!['GET', 'POST'].includes(config.method)) {
      throw createError(400, 'Config field "method" must be GET or POST');
    }
  }

  if (!isSimpleHeadersObject(finalHeaders)) {
    throw createError(400, 'Config field "headers" must be an object of string values');
  }

  if (config.json_body === undefined) {
    config.json_body = {};
  }
  if (typeof config.json_body === 'string') {
    try {
      config.json_body = JSON.parse(config.json_body);
    } catch (error) {
      throw createError(400, 'Config field "json_body" must be valid JSON', error.message);
    }
  }
  if (!isPlainObject(config.json_body)) {
    throw createError(400, 'Config field "json_body" must be a JSON object');
  }

  validateIntegerField(config, 'requests');
  validateIntegerField(config, 'concurrency');
  validateIntegerField(config, 'duration');
  validateIntegerField(config, 'rampUp');
  validateIntegerField(config, 'loops');
  validateIntegerField(config, 'request_timeout');
  validateIntegerField(config, 'rps');
  validateIntegerField(config, 'retries');

  validateOptionalString(config, 'warmup');

  if (config.requests === undefined) {
    throw createError(400, 'Config field "requests" is required');
  }

  if (config.concurrency === undefined) {
    throw createError(400, 'Config field "concurrency" is required');
  }

  if (config.concurrency <= 0) {
    throw createError(400, 'Config field "concurrency" must be greater than zero');
  }

  if ((config.duration ?? 0) < 0) {
    throw createError(400, 'Config field "duration" must be zero or greater');
  }

  if ((config.rampUp ?? 0) < 0) {
    throw createError(400, 'Config field "rampUp" must be zero or greater');
  }

  if ((config.loops ?? 0) < 0) {
    throw createError(400, 'Config field "loops" must be zero or greater');
  }

  if ((config.request_timeout ?? 0) < 0) {
    throw createError(400, 'Config field "request_timeout" must be zero or greater');
  }

  if ((config.duration ?? 0) > 0 && (config.rampUp ?? 0) > (config.duration ?? 0)) {
    throw createError(400, 'Config field "rampUp" must be less than or equal to duration');
  }

  return {
    url: config.url.trim(),
    method: config.method || 'GET',
    headers: finalHeaders,
    json_body: config.json_body,
    requests: config.requests,
    concurrency: config.concurrency,
    duration: config.duration ?? 0,
    rampUp: config.rampUp ?? 0,
    loops: config.loops ?? 0,
    request_timeout: config.request_timeout ?? 5,
    rps: config.rps ?? 0,
    retries: config.retries ?? 0,
    warmup: config.warmup || '5s',
  };
}

function validateIntegerField(config, field) {
  if (config[field] === undefined) {
    return;
  }

  if (!Number.isInteger(config[field])) {
    throw createError(400, `Config field "${field}" must be an integer`);
  }
}

function validateOptionalString(config, field) {
  if (config[field] === undefined) {
    return;
  }

  if (typeof config[field] !== 'string') {
    throw createError(400, `Config field "${field}" must be a string`);
  }
}

function isSimpleHeadersObject(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false;
  }

  return Object.values(value).every((entry) => typeof entry === 'string');
}

function isPlainObject(value) {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function mapExecutionError(error) {
  if (error && error.statusCode) {
    if (error.details && typeof error.details.stderr === 'string') {
      const parsed = parseStructuredGoError(error.details.stderr);
      if (parsed) {
        error.publicMessage = parsed.error || error.publicMessage;
        error.details = parsed.details || error.details.stderr;
      }
    }
    return error;
  }

  if (error && error.code === 'ENOENT') {
    return createError(500, 'Go load test command not found', {
      command: GO_BINARY_PATH || 'go',
      hint: GO_BINARY_PATH
        ? 'Verify GO_BINARY_PATH points to an executable on the server.'
        : 'Install Go on the server or set GO_BINARY_PATH to a compiled load test binary.'
    });
  }

  if (error instanceof SyntaxError) {
    return createError(500, 'Load test completed but results file was invalid JSON', {
      details: error.message
    });
  }

  return createError(500, 'Unexpected load test execution failure', {
    details: error && error.message ? error.message : String(error)
  });
}

function createError(statusCode, publicMessage, details) {
  const error = new Error(publicMessage);
  error.statusCode = statusCode;
  error.publicMessage = publicMessage;
  error.details = details;
  return error;
}

function describeCommand() {
  return `${GO_BINARY_PATH} -config <temp-config>`;
}

function parseStructuredGoError(stderr) {
  const lines = stderr
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);

  for (let index = lines.length - 1; index >= 0; index--) {
    try {
      const parsed = JSON.parse(lines[index]);
      if (parsed && typeof parsed === 'object' && parsed.status === 'failed') {
        return parsed;
      }
    } catch {
      continue;
    }
  }

  return null;
}
