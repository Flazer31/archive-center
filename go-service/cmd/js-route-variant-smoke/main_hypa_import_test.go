package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestArchiveCenterJSHypaImportOriginalAndPartialResult(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node required for runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{}
	for _, signature := range []string{
		"function normalizeHypaImportSourceTurnIndex(",
		"async function importHypaMemory(",
		"async function explorerFetchMemories(",
		"function renderHypaImportResult(",
	} {
		blocks = append(blocks, extractJSFunctionBlockForTest(t, src, signature))
	}
	script := `
const assert = require('node:assert/strict');
const escapeAttr = value => String(value).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const t = key => key;
const _hypaImportState = {};
const _explorer = {memories: {items: [], offset: 0}, selectedSessionId: 'session-hypa'};
const EXPLORER_PAGE_SIZE = 30;
const explorerSessionId = () => _explorer.selectedSessionId;
const getCurrentChatSessionId = async () => 'session-hypa';
const captureSessionHostContextFromCache = sid => ({sid});
const R = {getChatFromIndex() {throw new Error('unexpected direct Host access');}};
const summaries = Array.from({length: 75}, (_, i) => ({text: '  Source ' + i + '\nsecond line  ', index: i}));
const resolveCurrentActiveChatObject = async (sid, ctx) => {
  assert.equal(sid, ctx.sid);
  return {chat: {hypaV3Data: {summaries}, message: [{role: 'char', data: 'RAW CHAT MUST NOT BE IMPORTED'}]}};
};
const showConfirmModal = async (title, message) => {
  assert(message.includes(String(summaries.length)));
  return true;
};
const buildAdminRuntimeClientMeta = value => value;
const safeCall = async fn => fn();
let refreshes = 0, imports = 0, reads = 0;
const refreshExplorerUI = () => {refreshes++;};
const alert = () => {throw new Error('unexpected alert');};
const bridgeFetch = async (route, options) => {
  if (route === '/import/hypamemory') {
    imports++;
    assert.equal(_hypaImportState.loading, true);
    assert.equal(options.method, 'POST');
    assert.equal(options.body.summaries.length, summaries.length);
    assert.deepEqual(options.body.summaries.map(s => s.text), summaries.map(s => s.text));
    return {status: 'partial_error', code: 'hypamemory_import', total: 75,
      saved: 73, existing: 1, failed: 1, skipped: 0,
      analysis_succeeded: 72, analysis_failed: 1, analysis_skipped: 1,
      items: [{index: 4, status: 'save_failed'}], errors: ['store <failure>']};
  }
  const url = new URL(route, 'http://test.invalid');
  assert.equal(url.pathname, '/explorer/memories');
  assert.equal(url.searchParams.get('chat_session_id'), 'session-hypa');
  assert.equal(url.searchParams.get('source'), reads === 0 ? 'hypamemory' : null);
  reads++;
  return {items: [{id: 1}], total: 74, has_more: true};
};
` + strings.Join(blocks, "\n") + `
(async () => {
  await importHypaMemory('session-hypa');
  assert.equal(imports, 1);
  assert.equal(reads, 1);
  assert.equal(_hypaImportState.loading, false);
  assert.equal(_hypaImportState.error, null);
  assert.equal(_hypaImportState.result.saved, 73);
  assert.equal(_hypaImportState.result.failed, 1);
  assert.equal(_explorer.memories.total, 74);
  assert(refreshes >= 2);
  const html = renderHypaImportResult(_hypaImportState.result);
  assert(html.includes('hypaImport.count.saved <b>73</b>'));
  assert(html.includes('hypaImport.count.failed <b>1</b>'));
  assert(html.includes('hypaImport.count.skipped <b>0</b>'));
  assert(html.includes('store &lt;failure&gt;'));
  assert(!html.includes('background') && !html.includes('bgNote'));
  const unknown = renderHypaImportResult({total: 75});
  assert(unknown.includes('hypaImport.count.saved <b>—</b>'));
  _explorer.memories.source = '';
  await explorerFetchMemories(true);
  assert.equal(reads, 2);
})().catch(error => { console.error(error); process.exitCode = 1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Hypa import runtime: %v\n%s", err, output)
	}
}
