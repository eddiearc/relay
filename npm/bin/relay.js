#!/usr/bin/env node

import { spawn } from 'node:child_process';
import { resolveBinaryPath } from '../lib/resolve-binary.mjs';

const binaryPath = resolveBinaryPath();
const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});

child.on('error', (error) => {
  console.error(error.message);
  process.exit(1);
});
