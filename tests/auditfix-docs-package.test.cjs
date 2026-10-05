'use strict';

// Text/copy-list regressions only. No plugin initialization, package build,
// network, provider, database, or installed-user files are exercised.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const root = path.resolve(__dirname, '..');
const read = relative => fs.readFileSync(path.join(root, relative), 'utf8');
// Operational notes stay outside release/source artifacts. A local verification
// run supplies its reviewed companion explicitly instead of depending on a
// developer's directory layout in distributed tests.
const companion = process.env.AC_AUDITFIX_GUARDRAILS_PATH;

function translations(key) {
  const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const values = [...read('Archive Center.js').matchAll(new RegExp(
    '"' + escaped + '"\\s*:\\s*("(?:[^"\\\\]|\\\\.)*")', 'g'))]
    .map(match => JSON.parse(match[1]));
  assert.equal(values.length, 3, `${key}: all ko/en/ja production strings are required`);
  return values;
}

test('P1-10: installation README permits managed migration updates and explains startup', () => {
  const text = read('ops/full-package/README_FULL_PACKAGE.md');
  assert.doesNotMatch(text, /rejects a package that adds or changes managed migration/i);
  assert.doesNotMatch(text, /database-changing releases require.*manual migration/is);
  assert.match(text, /(?:allow|support)\w*[^.]*managed migration SQL[^.]*mariadb-schema\.exe/is);
  assert.match(text, /launcher[^.]*mariadb-schema\.exe[^.]*migration/is);
  assert.match(text, /ready/);
  assert.match(text, /version/);
  for (const retained of ['.runtime/', '.updates/', '.env.full.local', '.env.full.local.protected']) {
    assert.ok(text.includes(retained), `preservation instructions missing: ${retained}`);
  }
});

test('P1-13: local operational companion treats K as priority count and keeps candidate scope', {skip: !companion && 'set AC_AUDITFIX_GUARDRAILS_PATH for the local document check'}, () => {
  const text = fs.readFileSync(companion, 'utf8');
  assert.doesNotMatch(text, /K is a maximum|final per-group core-memory item ceiling|no character-budget fill after each group's K/i);
  assert.doesNotMatch(text, /Apply the same UI maximum independently|do not transfer unused item slots/i);
  assert.match(text, /K is a priority (?:count|target)/i);
  assert.match(text, /remaining candidates[^.]*character (?:budget|ceiling)/i);
  assert.match(text, /candidate.reading[^.]*final injection/i);
  assert.match(text, /ceilings[^.]*fill targets/i);
  assert.match(text, /local companion/i);
});

test('P1-13: all K hints distinguish priority count from final maximum', () => {
  const values = translations('settings.label.coreObjectiveMemoryMaxItems.hint');
  const rules = [/우선.*최종.*상한/, /prioritiz.*not.*final.*maximum/i, /優先.*最終.*上限/];
  values.forEach((text, i) => assert.match(text, rules[i]));
});

const criticRules = [
  {identity: /식별표/, independent: /별도.*같은.*상한/, sum: /합계/, zero: /0.*ledger.*활성/, originals: /현재.*직전.*원문.*별개/, total: /전체 요청.*상한.*아닙/},
  {identity: /identity index/i, independent: /separate.*same.*ceiling/i, sum: /combined/i, zero: /0.*ledger.*enabled/i, originals: /current.*previous.*originals.*separate/i, total: /not.*total.*request.*limit/i},
  {identity: /識別表/, independent: /別.*同じ.*上限/, sum: /合計/, zero: /0.*ledger.*有効/, originals: /現在.*直前.*原文.*別/, total: /リクエスト全体.*上限.*では/},
];
for (const [i, locale] of ['ko', 'en', 'ja'].entries()) {
  test(`P1-14: ${locale} Critic hint includes independent identity cost, ledger and conditional zero`, () => {
    const text = translations('settings.hint.criticReferenceMaxChars')[i];
    for (const [claim, rule] of Object.entries(criticRules[i])) assert.match(text, rule, claim);
  });
}

test('Package P1-2: execute production Windows/POSIX copy pipelines on synthetic files', {skip: process.platform !== 'win32' && 'Windows PowerShell AST execution required'}, () => {
  const args = ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File',
    path.join(root, 'ops/package-runtime-copy-list-test.ps1'), '-RepoRoot', root];
  const scratch = process.env.AC_COPY_LIST_TEST_TEMP;
  if (scratch) args.push('-ScratchParent', scratch);
  const result = spawnSync('powershell.exe', args, {encoding: 'utf8', windowsHide: true});
  assert.ifError(result.error);
  process.stdout.write(result.stdout || '');
  assert.equal(result.status, 0, result.stderr || result.stdout);
});
