# AC Recomposer Agent Beta 0.01 소개글

Layout archetype: magazine feature / editorial proof sheet
Macro shape: full-width document with numbered proof lanes
Color temperature: warm neutral paper with ink-blue and correction-red accents
Density: balanced
Border language: sharp / double-rule editorial
Typography mood: authoritative but approachable
Primary material: paper and printer's ink

아래 HTML 블록은 아카라이브 글쓰기 화면에 붙여넣는 용도로 작성했습니다.

<div style="background:#eee6d7;color:#29251f;font-family:'Malgun Gothic','Apple SD Gothic Neo',Arial,sans-serif;line-height:1.8;padding:18px;border:1px solid #5a5146;box-shadow:inset 0 0 34px rgba(70,54,34,0.12),0 8px 24px rgba(42,35,27,0.22);">
  <div style="background:#24364b;color:#f6f0e4;padding:24px;border:1px solid #172536;box-shadow:inset 0 0 18px rgba(0,0,0,0.24);">
    <div style="display:table;width:100%;">
      <div style="display:table-cell;vertical-align:middle;padding-right:16px;">
        <div style="font-size:11px;letter-spacing:4px;color:#d8c9a8;font-weight:800;">RISUAI AGENT PLUGIN &middot; OPEN BETA</div>
        <div style="font-size:34px;font-weight:900;line-height:1.2;color:#fffaf0;margin-top:6px;text-shadow:0 2px 8px rgba(0,0,0,0.3);">AC Recomposer Agent</div>
        <div style="font-size:17px;font-weight:800;color:#f1dfbd;margin-top:10px;">한 번 나온 답변을, 편집 회의 한 번 더</div>
      </div>
      <div style="display:table-cell;width:150px;vertical-align:middle;text-align:center;border:2px solid #d7695d;background:#f4ead8;padding:12px;box-shadow:0 0 0 3px rgba(215,105,93,0.18);">
        <div style="font-size:11px;letter-spacing:3px;color:#8c2f2b;font-weight:900;">FIELD TEST</div>
        <div style="font-size:26px;line-height:1.2;color:#8c2f2b;font-weight:900;margin-top:4px;">Beta 0.01</div>
        <div style="font-size:11px;color:#6f514a;margin-top:4px;">사용 경험 수집판</div>
      </div>
    </div>
    <div style="margin-top:18px;border-top:1px solid #66778a;padding-top:14px;font-size:14px;color:#e7e0d5;">
      RisuAI가 만든 원본 답변을 여러 전문 역할이 <strong style="color:#ffffff;">인물, 관계, 사건, 세계관, 문체, 비밀, 시점</strong>의 관점에서 다시 읽고, 검증된 개선점만 한 편의 최종 장면으로 재구성하는 독립형 플러그인입니다.
    </div>
  </div>

  <div style="margin-top:16px;background:#f8f2e7;border:2px solid #a33b32;padding:18px 20px;box-shadow:inset 5px 0 0 #a33b32;">
    <div style="font-size:12px;letter-spacing:3px;color:#8c2f2b;font-weight:900;">WHY OPEN BETA?</div>
    <div style="font-size:21px;font-weight:900;color:#2a2520;margin-top:4px;">이번 배포의 목적은 &ldquo;완성 선언&rdquo;보다 실제 사용 체감을 모으는 것입니다.</div>
    <div style="font-size:14px;color:#52493f;margin-top:9px;">
      문장이 정말 더 좋아졌는지, 인물성과 관계가 더 또렷해졌는지, 기다린 시간과 호출 비용만큼 가치가 있었는지를 확인하고 싶습니다. 긴 보고서가 아니어도 괜찮습니다. <strong style="color:#8c2f2b;">&ldquo;원본이 더 좋았다&rdquo;, &ldquo;대사는 좋아졌지만 장면이 길어졌다&rdquo;</strong> 같은 한두 문장도 중요한 베타 자료가 됩니다.
    </div>
  </div>

  <div style="margin-top:24px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">EDITORIAL WORKFLOW</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;line-height:1.35;">답변 하나가 최종 장면이 되기까지</div>
    <div style="font-size:13px;color:#6a6054;margin-top:5px;">사용자 입력은 고쳐 쓰지 않고, 현재 턴의 근거를 정리한 뒤 모델 답변의 수정 가능한 문장만 편집합니다.</div>
  </div>

  <div style="margin-top:14px;border:1px solid #b8aa94;background:#f8f2e7;">
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:58px;vertical-align:middle;text-align:center;background:#24364b;color:#fff7e8;font-size:22px;font-weight:900;padding:14px 8px;">01</div>
      <div style="display:table-cell;vertical-align:middle;padding:13px 17px;">
        <div style="font-size:15px;font-weight:900;color:#26384c;">입력 전 정리</div>
        <div style="font-size:13px;color:#5c5349;margin-top:3px;">현재 대화, 캐릭터, 페르소나, 활성 로어북과 현재 채팅에 이미 저장된 Summary&middot;Supa&middot;Hypa 요약 필드를 확인하고 이번 턴의 사실&middot;비밀&middot;관계&middot;장면 목표를 짧은 작업 지시로 정리합니다. 원래 사용자 문장은 그대로 둡니다.</div>
      </div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:58px;vertical-align:middle;text-align:center;background:#31465e;color:#fff7e8;font-size:22px;font-weight:900;padding:14px 8px;">02</div>
      <div style="display:table-cell;vertical-align:middle;padding:13px 17px;">
        <div style="font-size:15px;font-weight:900;color:#26384c;">장면 전체 재작성 후보</div>
        <div style="font-size:13px;color:#5c5349;margin-top:3px;">선택된 전문 역할들이 조각난 조언이 아니라, 처음부터 끝까지 읽히는 완성된 장면 후보를 각자의 관점으로 작성합니다.</div>
      </div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:58px;vertical-align:middle;text-align:center;background:#3f5369;color:#fff7e8;font-size:22px;font-weight:900;padding:14px 8px;">03</div>
      <div style="display:table-cell;vertical-align:middle;padding:13px 17px;">
        <div style="font-size:15px;font-weight:900;color:#26384c;">의미 판정</div>
        <div style="font-size:13px;color:#5c5349;margin-top:3px;">Semantic Judge가 후보마다 좋아진 점, 누락, 근거 없는 추가, 비밀 누설, 시점 위반과 사용자 행동 강제를 구분합니다. 단순히 점수가 높은 후보 하나를 고르지는 않습니다.</div>
      </div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:58px;vertical-align:middle;text-align:center;background:#6d4d46;color:#fff7e8;font-size:22px;font-weight:900;padding:14px 8px;">04</div>
      <div style="display:table-cell;vertical-align:middle;padding:13px 17px;">
        <div style="font-size:15px;font-weight:900;color:#66332f;">한 명의 최종 집필자</div>
        <div style="font-size:13px;color:#5c5349;margin-top:3px;">Whole-Scene Composer만 최종 문장을 씁니다. 승인된 장점들을 이어 붙이는 대신, 사실과 장면 흐름을 유지하면서 하나의 자연스러운 글로 다시 구성합니다.</div>
      </div>
    </div>
    <div style="display:table;width:100%;">
      <div style="display:table-cell;width:58px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:22px;font-weight:900;padding:14px 8px;">05</div>
      <div style="display:table-cell;vertical-align:middle;padding:13px 17px;">
        <div style="font-size:15px;font-weight:900;color:#8c2f2b;">최종 검증 또는 원본 복귀</div>
        <div style="font-size:13px;color:#5c5349;margin-top:3px;">Final Semantic Prover가 사실, 장면의 핵심 진행, 비밀, 시점, 사용자 주도권과 출력 형식을 다시 확인합니다. 제한된 보정으로도 통과하지 못하면 수정본을 억지로 내보내지 않고 원본 답변을 돌려줍니다.</div>
      </div>
    </div>
  </div>

  <div style="margin-top:18px;background:#fff8eb;border:2px solid #b78745;padding:18px;box-shadow:inset 5px 0 0 #b78745;">
    <div style="font-size:11px;letter-spacing:3px;color:#8a5a24;font-weight:900;">WHAT DOES &ldquo;MEMORY&rdquo; MEAN?</div>
    <div style="font-size:20px;font-weight:900;color:#3a2d20;margin-top:4px;">이 플러그인에서 &ldquo;기억을 읽는다&rdquo;는 뜻</div>
    <div style="font-size:13px;color:#5c4b3a;margin-top:7px;">AC Recomposer Agent는 <strong style="color:#704138;">현재 RisuAI 채팅 안에 다른 기능이 이미 저장해 둔 요약 필드</strong>를 보조 문맥으로 읽습니다.</div>

    <table style="width:100%;border-collapse:collapse;margin-top:13px;font-size:13px;">
      <tbody>
        <tr>
          <td style="width:24%;vertical-align:top;border:1px solid #c9aa7d;background:#efe0c7;color:#704138;padding:11px;font-weight:900;">읽는 범위</td>
          <td style="vertical-align:top;border:1px solid #c9aa7d;background:#fffaf1;color:#5c4b3a;padding:11px;">현재 채팅의 <span style="font-family:monospace;color:#704138;">summary</span>, <span style="font-family:monospace;color:#704138;">note</span>, SupaMemory, HypaMemory V2&middot;V3, HypaMemory, LastMemory 필드를 읽습니다. 필드가 없으면 기억도 불러오지 않습니다.</td>
        </tr>
        <tr>
          <td style="width:24%;vertical-align:top;border:1px solid #c9aa7d;background:#efe0c7;color:#704138;padding:11px;font-weight:900;">HypaMemory V3</td>
          <td style="vertical-align:top;border:1px solid #c9aa7d;background:#fffaf1;color:#5c4b3a;padding:11px;">현재 채팅에 저장된 <span style="font-family:monospace;color:#704138;">hypaV3Data</span>가 있으면 그 요약을 읽습니다. 현재 구현에서는 각 요약 필드의 최대 600자만 사용하며, HypaMemory V3의 DB를 직접 검색하거나 수정하지 않습니다.</td>
        </tr>
      </tbody>
    </table>

    <div style="font-size:12px;color:#765f45;margin-top:10px;border-left:3px solid #b78745;padding-left:10px;">따라서 이 플러그인에서 말하는 기억은 <strong style="color:#704138;">현재 채팅에 이미 존재하는 요약 스냅샷</strong>을 불러오는 범위로 한정됩니다.</div>
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">FOUR EDITORIAL LENSES</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">무엇을 보고 고치나요?</div>
  </div>

  <table style="width:100%;border-collapse:collapse;margin-top:14px;">
    <tbody>
      <tr>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:16px;">
          <div style="font-size:11px;color:#8c2f2b;font-weight:900;letter-spacing:2px;">CHARACTER</div>
          <div style="font-size:17px;color:#26384c;font-weight:900;margin-top:3px;">인물과 관계</div>
          <div style="font-size:13px;color:#5c5349;margin-top:6px;">말투, 감정의 자세, 서로를 대하는 거리, 알아보는 방식과 대사의 속뜻을 점검합니다.</div>
        </td>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f3ecdf;padding:16px;">
          <div style="font-size:11px;color:#8c2f2b;font-weight:900;letter-spacing:2px;">CONTINUITY</div>
          <div style="font-size:17px;color:#26384c;font-weight:900;margin-top:3px;">사건과 세계</div>
          <div style="font-size:13px;color:#5c5349;margin-top:6px;">앞뒤 인과, 현재 위치와 사물 상태, 활성 설정, 장면 진행과 물리적 연속성을 확인합니다.</div>
        </td>
      </tr>
      <tr>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f3ecdf;padding:16px;">
          <div style="font-size:11px;color:#8c2f2b;font-weight:900;letter-spacing:2px;">PROSE</div>
          <div style="font-size:17px;color:#26384c;font-weight:900;margin-top:3px;">문체와 극적 힘</div>
          <div style="font-size:13px;color:#5c5349;margin-top:6px;">문장 리듬, 이미지, 반복, 장면 압력, 전환, 설명투와 마지막 문단의 여운을 다듬습니다.</div>
        </td>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:16px;">
          <div style="font-size:11px;color:#8c2f2b;font-weight:900;letter-spacing:2px;">BOUNDARY</div>
          <div style="font-size:17px;color:#26384c;font-weight:900;margin-top:3px;">비밀과 경계</div>
          <div style="font-size:13px;color:#5c5349;margin-top:6px;">누가 무엇을 아는지, 정체 공개 시점, 관찰 가능한 범위, 사용자 캐릭터의 선택권과 메타 문구를 검사합니다.</div>
        </td>
      </tr>
    </tbody>
  </table>

  <div style="margin-top:18px;padding:18px;background:#24364b;color:#eee7db;border-left:6px solid #d7695d;box-shadow:inset 0 0 16px rgba(0,0,0,0.2);">
    <div style="font-size:12px;letter-spacing:3px;color:#f0b0a9;font-weight:900;">핵심 차이</div>
    <div style="font-size:18px;font-weight:900;color:#fffaf0;margin-top:4px;">여러 답변 중 하나를 뽑는 시스템이 아니라, 여러 답변에서 검증된 장점만 모아 새 최종문을 만드는 시스템입니다.</div>
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">EXPECTED RESULTS</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">어느 정도의 변화를 기대할 수 있나요?</div>
    <div style="font-size:13px;color:#6a6054;margin-top:5px;">원본 장면의 사실과 사용자 선택권을 유지하면서, 표현과 장면 구성의 완성도를 높이는 것이 목표입니다.</div>
  </div>

  <table style="width:100%;border-collapse:collapse;margin-top:14px;">
    <tbody>
      <tr>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:15px;">
          <div style="font-size:15px;font-weight:900;color:#26384c;">인물성이 더 또렷한 대사</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">모든 인물이 비슷하게 말하는 현상을 줄이고 관계의 거리와 감정의 속뜻이 드러나는 방향을 기대할 수 있습니다.</div>
        </td>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f3ecdf;padding:15px;">
          <div style="font-size:15px;font-weight:900;color:#26384c;">덜 끊기는 사건 진행</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">위치, 사물, 직전 행동과 장면의 인과를 함께 확인해 갑작스러운 비약이나 앞뒤 불일치를 줄이는 것이 목표입니다.</div>
        </td>
      </tr>
      <tr>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f3ecdf;padding:15px;">
          <div style="font-size:15px;font-weight:900;color:#26384c;">다듬어진 문체와 여운</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">반복되는 설명, 밋밋한 전환과 급하게 끝나는 마지막 문단을 줄이고 장면 전체의 리듬을 정리합니다.</div>
        </td>
        <td style="width:50%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:15px;">
          <div style="font-size:15px;font-weight:900;color:#26384c;">비밀&middot;시점&middot;선택권 보호</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">인물이 알 수 없는 사실, 아직 공개되지 않은 정체와 사용자가 정하지 않은 행동이 최종문에 섞이는 일을 줄이도록 검사합니다.</div>
        </td>
      </tr>
    </tbody>
  </table>

  <div style="margin-top:12px;background:#efe2cd;border:1px solid #b89d7b;padding:14px 16px;font-size:13px;color:#5c5349;">
    <strong style="color:#704138;">보장하지 않는 것:</strong> 모든 턴에서 원본보다 반드시 좋은 결과, 원본과 동일한 길이와 분위기, 현재 문맥에 없는 설정의 보충은 보장하지 않습니다. 최종 판단은 사용자의 취향과 장면 목적에 따라 달라질 수 있습니다.
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">DO NOT EDIT</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">고치면 안 되는 출력은 따로 보존합니다</div>
    <div style="font-size:13px;color:#6a6054;margin-top:5px;">감지된 보호 구간은 재작성 대상에서 분리하고, 사용자가 추가 보호 정규식을 지정할 수도 있습니다.</div>
  </div>

  <div style="margin-top:12px;background:#f8f2e7;border:1px solid #b8aa94;padding:16px;">
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">이미지 태그</span>
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">Markdown 이미지</span>
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">코드 블록</span>
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">Risu 마커</span>
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">HTML 태그</span>
    <span style="display:inline-block;background:#e4d7c1;color:#553d31;border:1px solid #b89d7b;padding:6px 10px;font-size:12px;font-weight:900;margin:3px;">상태창과 표</span>
    <div style="font-size:13px;color:#5c5349;margin-top:10px;">이미지 명령이나 상태 표시가 섞인 캐릭터를 사용한다면, 베타에서 특히 이 부분이 그대로 유지되는지 확인해 주세요.</div>
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">RUNTIME PRESETS</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">처음에는 Balanced를 권장합니다</div>
    <div style="font-size:13px;color:#6a6054;margin-top:5px;">모든 프리셋은 설정된 역할만 호출합니다. 더 높은 프리셋일수록 대기 시간과 사용량이 늘어날 수 있습니다.</div>
  </div>

  <table style="width:100%;border-collapse:collapse;margin-top:14px;font-size:13px;">
    <thead>
      <tr>
        <th style="width:20%;text-align:left;background:#24364b;color:#fff7e8;border:1px solid #182638;padding:11px 12px;">프리셋</th>
        <th style="width:48%;text-align:left;background:#24364b;color:#fff7e8;border:1px solid #182638;padding:11px 12px;">구성</th>
        <th style="width:32%;text-align:left;background:#24364b;color:#fff7e8;border:1px solid #182638;padding:11px 12px;">추천 상황</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td style="vertical-align:top;background:#f8f2e7;color:#26384c;border:1px solid #b8aa94;padding:12px;font-weight:900;">Fast</td>
        <td style="vertical-align:top;background:#f8f2e7;color:#5c5349;border:1px solid #b8aa94;padding:12px;">입력 플래너 호출 없이 최대 2개의 재작성 관점 사용, 추가 수정 회차 없음</td>
        <td style="vertical-align:top;background:#f8f2e7;color:#5c5349;border:1px solid #b8aa94;padding:12px;">첫 작동 확인, 비교적 빠른 대화</td>
      </tr>
      <tr>
        <td style="vertical-align:top;background:#efe4d2;color:#8c2f2b;border:1px solid #b8aa94;padding:12px;font-weight:900;">Balanced</td>
        <td style="vertical-align:top;background:#efe4d2;color:#5c5349;border:1px solid #b8aa94;padding:12px;">최대 2개의 입력 계획 역할, 최대 3개의 재작성 관점, 필요한 경우 1회 추가 수정</td>
        <td style="vertical-align:top;background:#efe4d2;color:#5c5349;border:1px solid #b8aa94;padding:12px;"><strong style="color:#8c2f2b;">기본 추천</strong> &middot; 품질과 대기 시간 비교</td>
      </tr>
      <tr>
        <td style="vertical-align:top;background:#f8f2e7;color:#26384c;border:1px solid #b8aa94;padding:12px;font-weight:900;">Quality</td>
        <td style="vertical-align:top;background:#f8f2e7;color:#5c5349;border:1px solid #b8aa94;padding:12px;">최대 3개의 입력 계획 역할, 4개의 재작성 관점, 최대 2회 추가 수정</td>
        <td style="vertical-align:top;background:#f8f2e7;color:#5c5349;border:1px solid #b8aa94;padding:12px;">중요 장면, 속도보다 결과 우선</td>
      </tr>
    </tbody>
  </table>

  <div style="margin-top:18px;background:#f8f2e7;border:1px solid #b8aa94;padding:16px 18px;">
    <div style="font-size:16px;font-weight:900;color:#26384c;">역할마다 다른 모델을 지정할 수 있습니다</div>
    <div style="font-size:13px;color:#5c5349;margin-top:6px;">OpenAI 호환, Ollama 호환, Anthropic, Gemini, Vertex, Custom 공급자를 지원하며 역할별 모델&middot;Endpoint&middot;API Key&middot;Fallback을 따로 설정할 수 있습니다. 꼭 최고가 모델로 전부 채울 필요는 없습니다. 실제로 어떤 역할에 어떤 모델을 썼을 때 만족도가 좋았는지도 중요한 베타 정보입니다.</div>
  </div>

  <div style="margin-top:26px;border-top:4px double #8c2f2b;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">KNOWN BETA ISSUES</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">아직 남아있는 문제</div>
    <div style="font-size:13px;color:#6a6054;margin-top:5px;">안전장치는 들어가 있지만, 다음 문제들은 이번 베타에서 실제 사용 경험을 통해 확인하고 개선해야 합니다.</div>
  </div>

  <div style="margin-top:14px;border:1px solid #b8aa94;background:#f8f2e7;">
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">01</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">속도와 비용:</strong> 한 번의 답변에 여러 모델이 호출되므로 단일 모델 대화보다 느리고 사용량이 많습니다. Quality는 특히 오래 걸릴 수 있습니다.</div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">02</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">모델 조합 의존성:</strong> 역할에 배치한 모델이 구조화 응답을 잘 지키지 못하면 후보 생성이나 검증이 실패하고 원본 답변으로 돌아갈 수 있습니다. 이 경우 사용자에게는 아무 변화가 없는 것처럼 보일 수 있습니다.</div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">03</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">요약 기억의 품질:</strong> Summary&middot;Supa&middot;Hypa 필드는 현재 연관성을 다시 계산하지 않고 비어 있지 않은 값을 읽습니다. 오래되거나 중복된 요약이 있으면 불필요한 문맥으로 섞일 수 있습니다.</div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">04</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">과잉 재작성:</strong> 문장을 실질적으로 다시 쓰도록 설계되어 있어 장면이 길어지거나, 지나치게 극적이 되거나, 사용자가 좋아하던 원본의 담백한 맛이 약해질 수 있습니다.</div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px solid #c8baa3;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">05</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">보호 패턴의 빈틈:</strong> 알려진 이미지, 코드, 마커와 상태창은 분리하지만 새로운 모듈의 특수 문법까지 자동으로 모두 알아내지는 못합니다. 필요한 경우 추가 보호 정규식을 지정해야 합니다.</div>
    </div>
    <div style="display:table;width:100%;">
      <div style="display:table-cell;width:48px;vertical-align:middle;text-align:center;background:#8c2f2b;color:#fff7e8;font-size:16px;font-weight:900;padding:12px 7px;">06</div>
      <div style="display:table-cell;vertical-align:middle;padding:12px 15px;font-size:13px;color:#5c5349;"><strong style="color:#704138;">긴 문맥과 실사용 조합:</strong> 문맥 상한을 넘으면 일부 자료가 잘릴 수 있고, 큰 로어북에는 현재 활성 여부가 확실하지 않은 후보가 섞일 수 있습니다. 다양한 브라우저와 Provider&middot;Model 조합의 체감 검증도 아직 충분하지 않습니다.</div>
    </div>
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">FIRST TEST</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">이 세 가지 장면으로 시험해 주세요</div>
  </div>

  <table style="width:100%;border-collapse:collapse;margin-top:14px;">
    <tbody>
      <tr>
        <td style="width:32%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:14px;text-align:center;">
          <div style="font-size:20px;font-weight:900;color:#8c2f2b;">A</div>
          <div style="font-size:14px;font-weight:900;color:#26384c;margin-top:3px;">대화가 많은 장면</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">말투와 관계의 거리감이 살아나는지</div>
        </td>
        <td style="width:36%;vertical-align:top;border:1px solid #b8aa94;background:#f3ecdf;padding:14px;text-align:center;">
          <div style="font-size:20px;font-weight:900;color:#8c2f2b;">B</div>
          <div style="font-size:14px;font-weight:900;color:#26384c;margin-top:3px;">설정과 비밀이 있는 장면</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">앞뒤 사실과 아는 사람의 범위가 지켜지는지</div>
        </td>
        <td style="width:32%;vertical-align:top;border:1px solid #b8aa94;background:#f8f2e7;padding:14px;text-align:center;">
          <div style="font-size:20px;font-weight:900;color:#8c2f2b;">C</div>
          <div style="font-size:14px;font-weight:900;color:#26384c;margin-top:3px;">이미지&middot;상태창 포함 장면</div>
          <div style="font-size:12px;color:#5c5349;margin-top:5px;">보호 구간이 손상되지 않는지</div>
        </td>
      </tr>
    </tbody>
  </table>

  <div style="margin-top:26px;background:#2c2823;color:#f4ecdf;padding:22px;border-top:5px solid #a33b32;box-shadow:inset 0 0 20px rgba(0,0,0,0.26);">
    <div style="font-size:11px;letter-spacing:4px;color:#e6a49d;font-weight:900;">BETA FEEDBACK DESK</div>
    <div style="font-size:26px;font-weight:900;color:#fff8eb;line-height:1.35;margin-top:4px;">가장 듣고 싶은 질문은 하나입니다</div>
    <div style="font-size:18px;font-weight:900;color:#f0d5a8;margin-top:9px;">기다린 시간과 호출 비용을 감수할 만큼, 결과가 실제로 좋아졌나요?</div>
    <div style="font-size:13px;color:#ddd2c3;margin-top:9px;">좋아졌다면 무엇이 좋아졌는지, 아니라면 어디에서 원본의 장점을 잃었는지 알려주세요. 성공 사례만큼 실패 사례도 중요합니다.</div>

    <table style="width:100%;border-collapse:collapse;margin-top:16px;font-size:13px;">
      <tbody>
        <tr>
          <td style="width:24%;border:1px solid #5b5146;background:#37322c;color:#f0b0a9;padding:10px;font-weight:900;">결과 선호</td>
          <td style="border:1px solid #5b5146;background:#302b26;color:#e7ddcf;padding:10px;">원본 / 수정본 / 상황에 따라 다름</td>
        </tr>
        <tr>
          <td style="width:24%;border:1px solid #5b5146;background:#37322c;color:#f0b0a9;padding:10px;font-weight:900;">좋아진 점</td>
          <td style="border:1px solid #5b5146;background:#302b26;color:#e7ddcf;padding:10px;">대사, 관계, 인과, 설정 유지, 문체, 긴장감, 결말 등</td>
        </tr>
        <tr>
          <td style="width:24%;border:1px solid #5b5146;background:#37322c;color:#f0b0a9;padding:10px;font-weight:900;">나빠진 점</td>
          <td style="border:1px solid #5b5146;background:#302b26;color:#e7ddcf;padding:10px;">장황함, 캐릭터 붕괴, 새 설정 추가, 비밀 누설, 사용자 행동 강제 등</td>
        </tr>
        <tr>
          <td style="width:24%;border:1px solid #5b5146;background:#37322c;color:#f0b0a9;padding:10px;font-weight:900;">사용성</td>
          <td style="border:1px solid #5b5146;background:#302b26;color:#e7ddcf;padding:10px;">프리셋, 대기 시간, 호출 비용, 오류와 원본 복귀 체감</td>
        </tr>
      </tbody>
    </table>

    <details style="margin-top:16px;">
      <summary style="background:#453c34;color:#f3d9ad;border:1px solid #6a5b4d;padding:12px 14px;font-size:13px;font-weight:900;list-style:none;">복사해서 쓸 수 있는 간단 피드백 양식</summary>
      <div style="background:#211e1a;color:#e9dfd2;border:1px solid #5b5146;border-top:none;padding:15px;font-family:monospace;font-size:12px;line-height:1.9;">
        사용 프리셋: Fast / Balanced / Quality<br>
        역할별 Provider / Model:<br>
        결과 선호: 원본 / 수정본 / 상황에 따라 다름<br>
        가장 좋아진 부분:<br>
        가장 아쉬운 부분:<br>
        대기 시간 체감:<br>
        이미지&middot;상태창&middot;형식 손상 여부:<br>
        실행 기록의 final state / reason:<br>
        한 줄 총평:
      </div>
    </details>

    <div style="margin-top:13px;font-size:12px;color:#cbbfb1;border-left:3px solid #a33b32;padding-left:10px;">오류 자료를 공유할 때는 API Key와 공개하고 싶지 않은 대화 내용을 반드시 지워 주세요. 설정 화면의 문장 비교는 현재 세션의 최근 main 턴만 보여주며, 전체 문장을 Trace 저장소에 남기지 않습니다.</div>
  </div>

  <div style="margin-top:26px;border-top:4px double #3d372f;padding-top:14px;">
    <div style="font-size:11px;letter-spacing:4px;color:#8c2f2b;font-weight:900;">QUICK START</div>
    <div style="font-size:24px;font-weight:900;color:#29251f;">설치와 첫 실행</div>
  </div>

  <div style="margin-top:13px;background:#f8f2e7;border:1px solid #b8aa94;padding:17px;">
    <div style="display:table;width:100%;border-bottom:1px dashed #b8aa94;padding-bottom:9px;margin-bottom:9px;">
      <div style="display:table-cell;width:34px;color:#8c2f2b;font-size:16px;font-weight:900;vertical-align:top;">1.</div>
      <div style="display:table-cell;color:#52493f;font-size:13px;vertical-align:top;">RisuAI의 <strong style="color:#26384c;">Plugin Settings &rarr; Import Plugin</strong>에서 첨부된 <strong style="color:#26384c;">AC Recomposer Agent.js</strong> 파일을 불러옵니다.</div>
    </div>
    <div style="display:table;width:100%;border-bottom:1px dashed #b8aa94;padding-bottom:9px;margin-bottom:9px;">
      <div style="display:table-cell;width:34px;color:#8c2f2b;font-size:16px;font-weight:900;vertical-align:top;">2.</div>
      <div style="display:table-cell;color:#52493f;font-size:13px;vertical-align:top;">AC Recomposer Agent 설정을 열고, 먼저 <strong style="color:#26384c;">Balanced</strong>를 선택한 뒤 사용할 역할의 Provider, Model, Endpoint와 API Key를 설정합니다. 첫 실행에는 <strong style="color:#26384c;">재작성 역할 1개 이상 + Semantic Judge + Whole-Scene Composer + Final Semantic Prover</strong>가 필요합니다.</div>
    </div>
    <div style="display:table;width:100%;">
      <div style="display:table-cell;width:34px;color:#8c2f2b;font-size:16px;font-weight:900;vertical-align:top;">3.</div>
      <div style="display:table-cell;color:#52493f;font-size:13px;vertical-align:top;">평소처럼 대화한 뒤 설정 화면의 <strong style="color:#26384c;">문장 비교</strong>와 <strong style="color:#26384c;">실행 기록</strong>에서 실제 적용 여부를 확인합니다.</div>
    </div>
  </div>

  <div style="margin-top:18px;background:#efe2cd;border:1px solid #b89d7b;padding:16px 18px;">
    <div style="font-size:15px;font-weight:900;color:#704138;">베타 사용 전 알아둘 점</div>
    <div style="font-size:13px;color:#5c5349;margin-top:6px;">이 플러그인은 한 번의 답변에 여러 모델 호출을 사용할 수 있으므로 일반적인 단일 응답보다 느리고 비용이 더 들 수 있습니다. 역할에 필요한 장면 문맥은 사용자가 설정한 Provider로 전송됩니다. 구조화 응답이나 검증이 실패하면 원본으로 돌아갈 수 있으며, 다양한 RisuAI&middot;Provider&middot;Model 조합의 실사용 체감을 모으는 것이 이번 베타의 목적입니다.</div>
  </div>

  <div style="margin-top:24px;text-align:center;background:#24364b;color:#f4ecdf;padding:24px;border:1px solid #172536;box-shadow:inset 0 0 18px rgba(0,0,0,0.22);">
    <div style="font-size:11px;letter-spacing:4px;color:#e6a49d;font-weight:900;">AC RECOMPOSER AGENT &middot; BETA 0.01</div>
    <div style="font-size:22px;font-weight:900;color:#fff8eb;margin-top:7px;">좋았던 장면보다, 왜 좋았는지 알려주세요.</div>
    <div style="font-size:14px;color:#d9d0c4;margin-top:8px;">한 줄 감상, 실패한 장면, 예상보다 오래 걸린 경험까지 모두 다음 베타를 만드는 자료가 됩니다.</div>
    <div style="display:inline-block;margin-top:15px;background:#f4ead8;color:#8c2f2b;border:2px solid #d7695d;padding:10px 22px;font-size:13px;font-weight:900;letter-spacing:1px;">첨부 파일: AC Recomposer Agent.js</div>
  </div>
</div>
