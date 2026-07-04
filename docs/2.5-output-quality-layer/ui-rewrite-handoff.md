# Risu Output Quality Layer 2.5 — UI 전면 재작성 인수인계 (Handoff)

> 목적: 새 대화(빈 컨텍스트)에서 `Risu Output Quality Layer 2.5.js`의 **UI만** 처음부터
> 사용자 친화적으로 재작성하기 위한 안전 작업 지침. 기능 로직은 절대 변경하지 않는다.

## 0. 절대 규칙 (Non-negotiables)
1. **대상 파일은 오직 하나:** `Risu Output Quality Layer 2.5.js` (레포 루트).
   다른 파일(`Archive Center.js`, `go-service/`, `contracts/` 등)은 절대 건드리지 않는다.
2. **기능 로직 불변경:** 설정 저장/읽기, 이벤트 바인딩, trace 렌더 데이터 흐름,
   AI 역할/모드 상수 등 동작 로직은 바꾸지 않는다. UI(모양/레이아웃/문구 표현)만 개선한다.
3. **연결 지점(계약) 보존:** 이벤트 핸들러가 의존하는 아래 식별자는 값/이름을 그대로 유지.
   - 요소 ID: `roql-app`, `roql-status`, `roql-enabled` 등 `roql-*` 계열
   - 속성: `data-action="save|refresh|clear-trace|reset|hide"`, `data-roql-tab="..."`
   - `renderCheckField(id, ...)` 등이 만들어내는 input의 id/name 규칙
   즉, **CSS와 마크업 구조는 새로 써도, 핸들러가 querySelector로 찾는 훅은 유지**해야 한다.
4. **한글 텍스트 훼손 금지 (중요):** 이 파일의 한글은 터미널에서 mojibake(예: `"?꾩껜"`)로
   보일 수 있다. 이는 콘솔 인코딩 문제일 수 있으므로, **한글 문자열을 손으로 다시 타이핑하지 말 것.**
   반드시 `read_file`로 원문을 그대로 읽어 `replace_in_file`의 SEARCH/REPLACE에서
   기존 한글 문자열을 **복붙(그대로 유지)**하고, 바꾸는 건 CSS/구조/영문 클래스명 위주로 한다.
   문구 자체를 바꿔야 한다면 사용자에게 새 문구를 먼저 확인받는다.

## 1. 지금까지 파악한 UI 구조
- 파일 규모: 약 13,438줄 / ~649KB. IIFE `(async () => { "use strict"; ... })()`.
- UI 관련 핵심:
  - `renderPanel(statusMessage)` — 약 11,220줄부터 시작. 하나의 큰 `<style>` 블록 +
    상단 타이틀/액션 버튼 + 상태줄 + 전역 스위치 + 탭 + 활성 탭 내용을 문자열로 조립.
  - `renderTabs()` — `TAB_DEFS`를 순회해 `data-roql-tab` 버튼 생성.
  - `renderGlobalEnabledSwitch()` — `roql-enabled` 체크 필드.
  - `renderActiveTab(last, lastTraceJson)` — `activePanelTab` 값에 따라 분기
    (`input_router`, `readers`, `decision`, `table_read`, `overview` 등).
  - `renderRoleProfiles(title, ROLE_GROUPS.xxx)`, `renderCheckField(...)`,
    `renderMetric`, `renderTrace*` 등 다수의 헬퍼 render 함수.
- 이벤트 바인딩: `data-action` / `data-roql-tab` / `roql-*` id 기준으로 패널에 위임 바인딩됨.
- 주요 상수(로직, 건드리지 말 것): `MODES`, `MODEL_TIERS`, `EXECUTION_MODES`,
  `ROUTING_PROFILES`, `OPERATION_PRESETS`, `AI_ROLE_IDS`, `ROLE_GROUPS`, `TAB_DEFS` 등.

## 2. 현재 CSS 요약 (재작성 시 참고 기준선)
- 앱 컨테이너 `#roql-app`: 폭 `min(1040px, 100vw-24px)`, 밝은 테마(`#f7f8fb`/`#18202f`),
  system-ui 폰트.
- 컴포넌트 클래스: `.roql-shell`, `.roql-top`, `.roql-title`, `.roql-actions`,
  `.roql-btn`(+`.roql-btn-primary`), `.roql-status`, `.roql-global`, `.roql-tabs`/`.roql-tab`
  (+`.roql-tab-active`), `.roql-grid`, `.roql-section`, `.roql-fields`, `.roql-field`,
  `.roql-check`, `.roql-note`, `.roql-help*`, `.roql-summary`/`.roql-metric`,
  `.roql-roles`/`.roql-role*`, `.roql-badge*`, `.roql-details*`, `.roql-trace*`.
- 반응형: `@media (max-width:980px)`, `@media (max-width:760px)`에서 그리드를 1열로.

## 3. 권장 작업 절차 (새 대화에서)
1. `read_file`로 `Risu Output Quality Layer 2.5.js`의 UI 구간을 **작은 조각으로 나눠 정독**
   (먼저 `renderPanel` ~ `renderActiveTab`, 이후 각 헬퍼 render 함수).
2. 재작성 범위 확정: 1차로 **`<style>` 블록만** 현대적으로 교체(가장 안전).
   이후 필요 시 마크업 구조를 개선하되, 2번 규칙(훅 보존)을 반드시 지킨다.
3. `replace_in_file`로 **국소 교체**. 한글 문자열은 SEARCH 블록에서 원문 그대로 복사.
4. 변경 후 문법 검증: `node --check "Risu Output Quality Layer 2.5.js"` 로 파싱 확인.
5. 가능하면 패널 HTML을 브라우저로 미리보기해 시각 확인 후 사용자에게 보고.

## 4. 디자인 방향(사용자 요청 요약)
- "사용자 친화적으로, 이해하기 쉽게." 기능은 그대로, 시각/정보 구조만 개선.
- 제안 방향(새 대화에서 사용자와 확정): 명확한 정보 위계(제목/설명/그룹핑),
  일관된 여백·타이포·버튼 스타일, 접근성(대비/포커스), 반응형 유지, 다크/라이트 정합.

## 5. 정리
- 이 대화에서 만든 임시 파일(`_ui_panel_head.txt`)은 이미 삭제함.
- 원본 소스는 아직 한 글자도 수정하지 않음.
