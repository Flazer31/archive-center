package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestYumiV1ArchiveReadContextUsesStoredModelOriginalWithoutMutatingDisplay(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Yumi Translator compatibility fixture")
		}
	}

	src := readArchiveCenterJS(t)
	buildContext := extractArchiveCenterJSAsyncFunction(t, src, "buildYumiV1ArchiveReadContext")
	script := buildContext + `
const zlib = require("zlib");
function assert(condition, message) { if (!condition) throw new Error(message); }
function marker(id, text) {
  return "<!-- yumi-tr:v1:" + id + ":start -->" + text + "<!-- yumi-tr:v1:" + id + ":end -->";
}
function record(model) { return JSON.stringify({v:1, model, status:"done", translatedAt:1}); }

(async function() {
  const payloadMessages = [
    {role:"assistant", content:"prefix " + marker("hot", "번역문 hot") + " suffix"},
    {role:"assistant", content:marker("plain", "번역문 plain")},
    {role:"assistant", content:marker("gzip", "번역문 gzip")},
    {role:"assistant", content:marker("missing", "번역문 missing")},
    {role:"assistant", content:marker("broken", "번역문 broken")},
    {role:"assistant", content:"already original"},
    {role:"user", content:marker("hot", "사용자 입력")},
  ];
  const activeMessages = [
    {role:"assistant", content:marker("hot", "활성 채팅 번역문"), raw:{data:"display stays"}},
  ];
  const payloadBefore = JSON.stringify(payloadMessages);
  const activeBefore = JSON.stringify(activeMessages);
  const chat = {scriptstate:{
    "$__yumi_tr.hot": record("model original hot"),
    "$__yumi_tr.plain": "u:" + record("model original plain"),
    "$__yumi_tr.gzip": "z:" + zlib.gzipSync(Buffer.from(record("model original gzip"), "utf8")).toString("base64"),
    "$__yumi_tr.broken": "not-json",
  }};

  const result = await buildYumiV1ArchiveReadContext(payloadMessages, activeMessages, chat);
  assert(result.payloadMessages[0].content === "prefix model original hot suffix", "hot record original was not restored");
  assert(result.payloadMessages[1].content === "model original plain", "u: record original was not restored");
  assert(result.payloadMessages[2].content === "model original gzip", "z: record original was not restored");
  assert(result.payloadMessages[3].content === "번역문 missing", "missing metadata did not preserve visible translation text");
  assert(result.payloadMessages[4].content === "번역문 broken", "malformed metadata did not preserve visible translation text");
  assert(result.payloadMessages[5] === payloadMessages[5], "unmarked assistant message was unnecessarily copied");
  assert(result.payloadMessages[6] === payloadMessages[6], "user message was altered");
  assert(result.activeMessages[0].content === "model original hot", "active-chat copy did not use stored original");
  assert(result.activeMessages[0].raw === activeMessages[0].raw, "active-chat raw observation was rewritten");
  assert(JSON.stringify(payloadMessages) === payloadBefore, "payload messages were mutated");
  assert(JSON.stringify(activeMessages) === activeBefore, "active-chat messages were mutated");
  assert(result.stats.markerBlocks === 6, "unexpected marker block count: " + result.stats.markerBlocks);
  assert(result.stats.modelSourceBlocks === 4, "unexpected original-source block count: " + result.stats.modelSourceBlocks);
  assert(result.stats.displayFallbackBlocks === 2, "unexpected display fallback block count: " + result.stats.displayFallbackBlocks);
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exitCode = 1;
});
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Yumi Translator compatibility fixture failed: %v\n%s", err, out)
	}
}

func TestBeforeRequestUsesYumiOriginalOnlyForArchiveReadPaths(t *testing.T) {
	src := readArchiveCenterJS(t)
	beforeRequest := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")

	for _, needle := range []string{
		`const yumiArchiveReadContext = await buildYumiV1ArchiveReadContext(`,
		`const archiveReadMessages = yumiArchiveReadContext.payloadMessages`,
		`messages: archiveReadActiveMessages`,
		`resolveContinuityTriggerInfo(userInput, archiveReadMessages, orchSessionId`,
		`tryPrepareTurn(orchSessionId, userInput, archiveReadMessages, continuityInfo`,
		`const recentContext = getHostContextMessages(archiveReadMessages)`,
		`const { payload: injectedPayload, injectionResult } = applyContextInjection(outgoingPayload, lastOrchResult)`,
	} {
		if !strings.Contains(beforeRequest, needle) {
			t.Errorf("beforeRequest Yumi original-source wiring missing %q", needle)
		}
	}
	if strings.Contains(beforeRequest, `outgoingPayload = extractedMessages.rebuild(archiveReadMessages)`) ||
		strings.Contains(beforeRequest, `outgoingPayload = archiveReadMessages`) {
		t.Fatal("Yumi internal read copy leaked into the outgoing Risu payload")
	}
}
