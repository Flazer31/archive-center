'use strict';
// Exercise production JS stream owners with a controlled
// HTTP/host boundary. Never contacts the user's backend, browser, or AI provider.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const http = require('node:http');
const { Readable } = require('node:stream');
const { createHash } = require('node:crypto');
const assert = require('node:assert/strict');
const sourcePath = process.env.AC_HUD_SOURCE || path.resolve(__dirname, '../Archive Center.js');
const source = fs.readFileSync(sourcePath, 'utf8');
const names = [
  'turnWorkflowHUDStreamFailure', 'cancelTurnWorkflowHUDStream',
  'cancelTurnWorkflowHUDPreviousStream', 'openTurnWorkflowHUDStream',
  'decodeTurnWorkflowHUDStreamLine', 'readTurnWorkflowHUDNDJSON',
  'consumeTurnWorkflowHUDStreamLine', 'consumeTurnWorkflowHUDStream',
  'consumeTurnWorkflowHUDPreviousStreamLine', 'consumeTurnWorkflowHUDPreviousStream',
  'primeTurnWorkflowHUD', 'startTurnWorkflowHUDWatch', 'startTurnWorkflowHUDPreviousWatch',
  'dismissTurnWorkflowHUD', 'dismissTurnWorkflowHUDPrevious',
  'finishTurnWorkflowHUDCurrentGeneration', 'stopTurnWorkflowHUDWatch',
];
function extract(name) {
  const re = new RegExp('\\n  (?:async )?function ' + name + '\\(');
  const match = re.exec(source);
  assert(match, 'production function missing: ' + name);
  const begin = match.index + 1;
  const end = source.indexOf('\n  }', begin);
  assert(end > begin, 'production function end missing: ' + name);
  const body = source.slice(begin, end + 4);
  new vm.Script(body);
  return { name, start_line: source.slice(0, begin).split('\n').length,
    sha256: createHash('sha256').update(body).digest('hex'), body };
}
const extracted = names.map(extract);
const production = extracted.map(x => x.body).join('\n');
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(condition, label) {
  const end = Date.now() + 2500;
  while (!condition()) {
    if (Date.now() > end) throw new Error('fixture did not reach: ' + label);
    await pause(5);
  }
}
function deferred() {
  let resolve;
  const promise = new Promise(r => { resolve = r; });
  return { promise, resolve };
}

async function fixture({ abortMode = 'direct', maxSockets = 20, render = () => null } = {}) {
  const entries = new Map(), accepted = [], clients = new Map(), logs = [], ui = [];
  const sockets = new Set();
  const agent = new http.Agent({ keepAlive: true, maxSockets });
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    accepted.push(url.pathname);
    if (url.pathname === '/probe') { res.end('ok'); return; }
    assert.equal(url.pathname, '/turn-workflow/events');
    const id = url.searchParams.get('request_id');
    const entry = entries.get(id);
    assert(entry, 'unexpected HTTP request: ' + id);
    entry.response = res;
    res.on('close', () => { entry.closed = true; });
    res.writeHead(entry.mode.startsWith('http_error') ? 503 : 200, { 'Content-Type': 'application/x-ndjson', 'Cache-Control': 'no-cache' });
    if (entry.mode.startsWith('http_error')) {
      res.write('controlled unavailable response\n');
      if (entry.mode === 'http_error_closed') res.end();
      return;
    }
    if (entry.mode === 'malformed') res.write('{ invalid ndjson }\n');
    else res.write(JSON.stringify({ request_id: id, status: entry.mode.startsWith('terminal') ? 'completed' : 'awaiting_final_output', revision: 1 }) + '\n');
    if (entry.mode === 'terminal') res.end();
  });
  server.on('connection', socket => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const base = 'http://127.0.0.1:' + server.address().port;

  function nativeFetch(url, init) {
    const parsed = new URL(url);
    assert.equal(parsed.origin, base, 'external network is prohibited');
    assert.equal(init.method, 'GET');
    const id = parsed.searchParams.get('request_id');
    const metric = { id, cancels: 0, aborts: 0, reads: 0, reader: null };
    clients.set(id, metric);
    return new Promise((resolve, reject) => {
      const req = http.request(url, { method: 'GET', agent }, res => {
        // The second mode models a host bridge whose AbortSignal association
        // ends at response headers; reader cancellation still reaches the socket.
        if (abortMode === 'headers_only') init.signal.removeEventListener('abort', abort);
        const response = new Response(Readable.toWeb(res), { status: res.statusCode });
        metric.response = response;
        resolve({ ok: response.ok, status: response.status, body: {
          cancel(reason) { return response.body.cancel(reason); },
          getReader() {
            const raw = response.body.getReader();
            const reader = {
              read() { metric.reads++; return raw.read(); },
              cancel(reason) { metric.cancels++; return raw.cancel(reason); },
            };
            metric.reader = reader;
            return reader;
          },
        } });
      });
      const abort = () => { metric.aborts++; req.destroy(new Error('fixture AbortSignal')); };
      init.signal.addEventListener('abort', abort, { once: true });
      req.on('error', reject);
      if (init.signal.aborted) abort();
      req.end();
    });
  }
  const context = vm.createContext({
    AbortController, TextDecoder, ReadableStream, Response,
    R: { nativeFetch }, settings: { bridgeUrl: base, webDirectBridgeEnabled: false },
    _turnWorkflowHUDWatchToken: 0, _turnWorkflowHUDActiveRequestId: '',
    _turnWorkflowHUDWatchRunning: false, _turnWorkflowHUDLastRevision: 0,
    _turnWorkflowHUDTerminalRequestId: '', _turnWorkflowHUDStreamReader: null,
    _turnWorkflowHUDStreamAbortController: null,
    _turnWorkflowHUDHostWarningsByRequestId: new Map(),
    TURN_WORKFLOW_HUD_CONTRACT: 'turn_workflow_hud.v3',
    _turnWorkflowHUDPreviousWatchToken: 0, _turnWorkflowHUDPreviousRequestId: '',
    _turnWorkflowHUDPreviousWatchRunning: false, _turnWorkflowHUDPreviousLastRevision: 0,
    _turnWorkflowHUDPreviousLastView: null, _turnWorkflowHUDPreviousStreamReader: null,
    _turnWorkflowHUDPreviousStreamAbortController: null,
    _turnWorkflowHUDRenderChain: Promise.resolve(),
    turnWorkflowHUDIsEnabled: () => true,
    dismissTurnWorkflowHUD: () => { throw new Error('unexpected disabled HUD path'); },
    resolveBridgeRuntimeRoute: value => { assert.equal(value, base); return { url: value }; },
    clearTurnWorkflowHUDTimer: () => ui.push('clear-timer'),
    renderTurnWorkflowHUDPrevious: view => {
      assert.equal(view.revision, 0, 'only the initial previous-HUD display is a controlled UI boundary');
      assert.equal(view.status, 'running');
      ui.push('prime-previous:' + view.request_id);
      return Promise.resolve();
    },
    turnWorkflowHUDHasHostWarning: () => false,
    queueTurnWorkflowHUDOperation: (_label, fn) => fn(),
    removeTurnWorkflowHUDDismissListeners: async () => { ui.push('remove-listeners'); },
    takeTurnWorkflowHUDDismissListenerIds: () => [],
    getTurnWorkflowHUDMainDocument: async () => null,
    ensureTurnWorkflowHUDRoot: async () => null,
    debugLog: (...args) => logs.push(args.map(String).join(' ')),
  });
  for (const lane of ['Current', 'Previous']) {
    context[lane === 'Current' ? 'consumeTurnWorkflowHUD' : 'consumeTurnWorkflowHUDPrevious'] = view => {
      if (view.revision === 0) { ui.push('prime:' + view.request_id); return true; }
      ui.push(view.request_id);
      context._turnWorkflowHUDRenderChain = render(view) || Promise.resolve();
      return true;
    };
  }
  vm.runInContext(production, context, { filename: sourcePath });
  const fields = lane => lane === 'current'
    ? { reader: '_turnWorkflowHUDStreamReader', running: '_turnWorkflowHUDWatchRunning' }
    : { reader: '_turnWorkflowHUDPreviousStreamReader', running: '_turnWorkflowHUDPreviousWatchRunning' };
  function start(id, mode = 'running', lane = 'current') {
    assert(!entries.has(id));
    entries.set(id, { id, mode, closed: false, response: null });
    if (lane === 'current') {
      context.primeTurnWorkflowHUD(id);
      context.startTurnWorkflowHUDWatch(id);
    } else context.startTurnWorkflowHUDPreviousWatch(id);
  }
  function cancel(lane = 'current') {
    context[lane === 'current' ? 'cancelTurnWorkflowHUDStream' : 'cancelTurnWorkflowHUDPreviousStream']();
  }
  function probe() {
    let done = false;
    const request = http.get(base + '/probe', { agent }, res => {
      res.resume();
      res.on('end', () => { done = true; });
    });
    request.on('error', error => logs.push('probe error: ' + error.message));
    return { get done() { return done; } };
  }
  async function close() {
    agent.destroy();
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
  }
  return { context, entries, clients, logs, ui, start, cancel, probe, close, fields,
    open: () => [...entries.values()].filter(e => e.response && !e.closed).length,
    pending: () => Object.values(agent.requests).reduce((n, list) => n + list.length, 0),
    accepted,
  };
}


const results=[];
async function run(name, work) {
  await work(); results.push(name); console.log('PASS '+name);
}
(async()=>{
 for(const lane of ['current','previous']) {
  for(const mode of ['terminal','terminal_held','malformed','render','http_error_open','http_error_closed']) {
   await run(lane+': closes '+mode,async()=>{
    const f=await fixture({abortMode:'headers_only',render:()=>{if(mode==='render')throw Error('controlled renderer error');}});
    try {
     f.start('event',mode==='render'?'running':mode,lane);
     await until(()=>f.entries.get('event').response&&!f.context[f.fields(lane).running]&&f.open()===0,'watcher exits and server sees closed body');
     assert.equal(f.context[f.fields(lane).reader],null);
    }finally{await f.close();}
   });
  }
  for(const abortMode of ['direct','headers_only']) {
   await run(lane+': stale renderer preserves replacement / '+abortMode,async()=>{
    const gate=deferred();const f=await fixture({abortMode,render:v=>v.request_id==='old'?gate.promise:null});
    try{
     f.start('old','running',lane);await until(()=>f.ui.includes('old'),'old render pending');
     f.start('new','running',lane);await until(()=>f.ui.includes('new'),'new registered');
     const owner=f.clients.get('new').reader;assert.equal(f.context[f.fields(lane).reader],owner);
     gate.resolve();await pause(30);
     assert.equal(f.context[f.fields(lane).reader],owner,'old continuation erased replacement');
     assert.equal(f.context[f.fields(lane).running],true);assert.equal(f.clients.get('new').cancels,0);
     f.cancel(lane);await until(()=>f.open()===0,'both streams closed without retained diagnostic handles');
    }finally{gate.resolve();await f.close();}
   });
  }
 }
 for(const action of ['dismiss_all','dismiss_previous','finish_current','stop_current']) {
  await run('independent lifecycle: '+action,async()=>{
   const f=await fixture({abortMode:'headers_only'});
   try{
    f.start('current');f.start('previous','running','previous');
    await until(()=>f.ui.includes('current')&&f.ui.includes('previous'),'both active');
    if(action==='dismiss_all')await f.context.dismissTurnWorkflowHUD();
    if(action==='dismiss_previous')f.context.dismissTurnWorkflowHUDPrevious('previous');
    if(action==='finish_current')f.context.finishTurnWorkflowHUDCurrentGeneration('current');
    if(action==='stop_current')f.context.stopTurnWorkflowHUDWatch('current',false);
    const closed=action==='dismiss_all'?['current','previous']:[action==='dismiss_previous'?'previous':'current'];
    await until(()=>closed.every(id=>f.entries.get(id).closed),'expected lanes close');
    for(const id of ['current','previous'].filter(id=>!closed.includes(id)))assert.equal(f.entries.get(id).closed,false);
   }finally{await f.close();}
  });
 }
 await run('repeated decode failure leaves next request deliverable',async()=>{
  const f=await fixture({abortMode:'headers_only',maxSockets:6});
  try{
   for(let i=0;i<8;i++){f.start('error-'+i,'malformed');await until(()=>f.entries.get('error-'+i).response&&!f.context._turnWorkflowHUDWatchRunning&&f.open()===0,'error cleanup '+i);}
   const probe=f.probe();await until(()=>probe.done,'next HTTP request finishes');assert.equal(f.pending(),0);
  }finally{await f.close();}
 });
 console.log(JSON.stringify({source_sha256:createHash('sha256').update(source).digest('hex'),checks:results.length,ai_calls:0,tests:results}));
})().catch(e=>{console.error(e);process.exitCode=1;});
