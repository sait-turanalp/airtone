#!/usr/bin/env node
// PostToolUse(Edit|Write): when a .go file changes, run `go build ./...` from the
// nearest module root and surface compile errors back to the agent (exit 2 + stderr).
// Fills the gap left by the global typecheck hook, which only covers .ts/.tsx/.rs.
// Leading-edge debounced (10s) so a burst of edits doesn't stack builds.
import { readFileSync, statSync, writeFileSync, existsSync } from 'node:fs';
import { execSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';

let raw = '';
try { raw = readFileSync(0, 'utf8'); } catch { process.exit(0); }
let data;
try { data = JSON.parse(raw); } catch { process.exit(0); }

const fp = data?.tool_input?.file_path || '';
if (!fp.endsWith('.go')) process.exit(0);

// Leading-edge debounce: skip if we built within the last 10s.
const stamp = join(tmpdir(), 'airtone-go-build.stamp');
const now = Date.now();
try {
  if (existsSync(stamp) && now - statSync(stamp).mtimeMs < 10_000) process.exit(0);
} catch {}
try { writeFileSync(stamp, String(now)); } catch {}

// Walk up to the nearest go.mod.
let dir = dirname(fp), root = null;
for (let i = 0; i < 25 && dir && dir !== '/'; i++) {
  if (existsSync(join(dir, 'go.mod'))) { root = dir; break; }
  dir = dirname(dir);
}
if (!root) process.exit(0);

try {
  execSync('go build ./...', { cwd: root, stdio: 'pipe', timeout: 90_000 });
  process.exit(0);
} catch (e) {
  const out = ((e.stderr?.toString() || '') + (e.stdout?.toString() || '')).trim();
  process.stderr.write(`go build failed after editing ${fp}:\n${out}\n`);
  process.exit(2);
}
