---
name: rethink
description: 코드베이스 종합검진 — 전수조사 + codex 병렬 검토 + "처음부터 다시 만든다면?" + 수정 계획
argument-hint: [scope or instructions]
---

# Rethink

기존 코드베이스를 밑바닥부터 다시 본다. 전체를 뒤지고, codex와 병렬로 검토하고, "처음부터 다시 만든다면?"이라는 질문에 답한다. 코드를 수정하지 않고 계획만 세운다.

## 언제 사용

- 코드가 어느 정도 쌓인 후 전체를 점검하고 싶을 때
- 크리티컬 이슈, 잠재 버그, 성능 문제를 전수조사할 때
- 과도한 구현, 미사용 코드, 설계 결함을 정리하고 싶을 때
- "이거 처음부터 다시 짠다면 어떻게 하겠어?"라는 질문이 들 때

## 사용하지 말 것

- 새 기능 아이디어 구체화 → `/ina:think`
- 방금 작성한 코드 리뷰 → `/ina:review`
- 이미 수정할 게 명확 → `/ina:plan`

## 인자

자연어 문자열. 없으면 전체 코드베이스를 검토한다.

- `rethink` — 전체 검토
- `rethink daemon/` — daemon 패키지만
- `rethink 테스트 가능성도 분석해줘` — 테스트 가능성 분석 포함
- `rethink look01.md도 참고해서 비교해줘` — 외부 문서 비교 포함

## ina 연동

- 스캔 시작: `ina_report_progress(in_progress="RETHINK: 전수조사")`
- codex 검토: `ina_report_progress(in_progress="RETHINK: codex 병렬 검토")`
- 종합: `ina_report_progress(in_progress="RETHINK: 종합 분석")`
- 막히면: `ina_mark_blocked(reason="RETHINK: {이유}")`
- 데몬 MCP 호출 실패 시 무시하고 계속 진행

## 전체 흐름

```
Phase 1: 전수조사 (자동 스캔, 질문 없이)
Phase 2: codex 병렬 검토 (독립적 시각)
Phase 3: 종합 분석 + "처음부터 다시 만든다면?"
Phase 4: 수정 계획 수립
```

**원칙: 코드를 수정하지 않는다. 분석하고 계획만 세운다.**

---

## Phase 1: 전수조사

> `ina_report_progress(in_progress="RETHINK: 전수조사")`

scope에 해당하는 코드를 전부 읽고 다음을 분류한다:

### 검토 항목

| 카테고리 | 찾는 것 |
|----------|---------|
| **Critical** | 버그, 데이터 손실 가능성, 보안 취약점 |
| **Performance** | 불필요한 반복, 비효율적 자료구조, 동시성 문제 |
| **Overengineering** | 과도한 추상화, 사용되지 않는 확장 포인트, YAGNI 위반 |
| **Dead code** | 미사용 함수, 미참조 상수, 도달 불가 분기 |
| **Structure** | 너무 긴 함수(50줄+), 너무 긴 파일(300줄+), 공통화 기회 |
| **Design** | 책임 혼재, 순환 의존, 잘못된 추상화 경계 |

### 스캔 방법

1. CLAUDE.md 읽어서 프로젝트 컨텍스트 확보
2. 디렉토리 구조 파악 (Glob)
3. 모든 소스 파일 순회하며 읽기
4. 각 파일에서 위 카테고리별 이슈 식별
5. 이슈마다 `{severity} | {file}:{line} | {description}` 형식으로 기록

### 테스트 가능성 분석 (인자에 요청 시)

- 테스트 없는 공개 함수/메서드 식별
- 테스트하기 어려운 구조 (전역 상태, 하드코딩된 의존성, 부수효과 은닉)
- 테스트 가능하게 바꾸려면 어떤 리팩토링이 필요한지

### 외부 참고 문서 비교 (인자에 파일 언급 시)

- 참고 문서를 읽고 핵심 패턴/구조 추출
- 현재 코드베이스와 비교하여 적용 가능한 패턴 식별
- 차이점과 도입 시 트레이드오프 분석

---

## Phase 2: codex 병렬 검토

> `ina_report_progress(in_progress="RETHINK: codex 병렬 검토")`

Phase 1과 **독립적인 시각**을 확보하기 위해 codex에 별도 검토를 위임한다.

```
codex-rescue Agent:
  "이 코드베이스를 검토해줘. 수정하지 말고 분석만.
   중점: 크리티컬 이슈, 잠재 버그, 성능 문제, 과도 구현,
   설계 결함, 미사용 코드. 파일:라인 단위로 보고.
   scope: {scope}"
```

codex 실패 시: Phase 1 결과만으로 진행 (codex는 보너스).

---

## Phase 3: 종합 분석

> `ina_report_progress(in_progress="RETHINK: 종합 분석")`

### >>> Step 1: Findings 병합

Phase 1 + Phase 2 결과를 병합:
- 동일 파일:라인 이슈 → 심각도 높은 쪽 유지
- 한쪽에서만 발견된 이슈 → 그대로 포함
- critical → high → medium → low 정렬

### >>> Step 2: "처음부터 다시 만든다면?"

지금까지 파악한 것들을 바탕으로:
1. 현재 설계의 **근본적 한계**는 무엇인가?
2. 처음부터 다시 만든다면 **어떤 구조**를 택하겠는가?
3. 코드 복잡성과 파편화를 줄이기 위한 **핵심 리팩토링**은?
4. 시작 전에 미리 해두었어야 할 작업은?

### >>> Step 3: 사용자 보고

findings 테이블 + 재설계 관점을 사용자에게 보고.
사용자 피드백을 받아 우선순위 조정.

---

## Phase 4: 수정 계획

> `ina_report_progress(in_progress="RETHINK: 수정 계획 수립")`

findings + 사용자 피드백을 바탕으로 수정 계획을 작성:

```markdown
## 수정 계획

### 즉시 수정 (Critical/High)
- [ ] {이슈}: {수정 방향} — {파일}

### 구조 개선
- [ ] {이슈}: {수정 방향} — {영향 범위}

### 선택적 개선 (Medium/Low)
- [ ] {이슈}: {수정 방향}
```

`.ina/specs/{YYYYMMDD-HHMM}-rethink-{slug}.md`에 저장.

---

## 실행 브릿지

수정 계획 완성 후 다음 단계 제안:
- `/ina:plan` — 수정 계획을 태스크로 분해 (추천)
- `/ina:build` — 단순한 수정이면 바로 실행
- 보류 — 계획만 남기고 나중에 실행

## 오동작 방지

- 각 Phase 진입 시 `>>> Rethink: {name}` 출력
- **코드를 절대 수정하지 않는다** — 분석과 계획만
- guard 규칙 자동 적용
- 데몬 MCP 호출 실패 시 무시하고 계속 진행

## 입출력

- **입력**: scope (선택), --testability (선택), --ref {file} (선택)
- **출력**: `.ina/specs/{YYYYMMDD-HHMM}-rethink-{slug}.md` (분석 보고서 + 수정 계획)
