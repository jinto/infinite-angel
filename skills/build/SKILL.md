---
name: build
description: ina 태스크 실행 — 직접, subagent, team 위임 + 테스트 검증
argument-hint: [--non-interactive]
---

# Build

TASKS.md의 미완료 태스크를 구현하는 순수 실행기. 태스크 성격에 따라 직접 실행, 서브에이전트 병렬, 또는 team을 자동 선택한다.

플래닝, 코드 리뷰, 커밋은 이 스킬의 범위가 아니다. autopilot 파이프라인에서 plan과 review가 별도로 담당한다.

## 언제 사용

- TASKS.md에 할 일이 정의되어 있고 구현할 때
- autopilot 파이프라인의 execute 단계로 호출될 때

## 사용하지 말 것

- 아이디어가 모호할 때 → `/ina:think`
- 플랜이 없을 때 → `/ina:plan`
- 코드 리뷰가 필요할 때 → `/ina:review`
- 처음부터 끝까지 자동 → `/ina:autopilot`

## 인자

- (없음): 인터랙티브 모드 (단계마다 확인)
- `--non-interactive`: 자동 진행 (문제 발생 시에만 멈춤)

## ina 연동

- 태스크 시작: `ina_report_progress(in_progress="BUILD: {태스크}", remaining="{남은 수}")`
- 막히면: `ina_mark_blocked(reason="BUILD: {이유}")`
- 데몬 MCP 호출 실패 시 무시하고 계속 진행

## 전체 흐름

```
┌──────────────────────────────────────────────────┐
│  1. 태스크 식별: TASKS.md에서 미완료 항목 파악       │
│  2. 실행 위임 판단: 직접 / subagent / team          │
│  3. 구현: 위임 방식에 따라 실행                     │
│  4. 검증: 테스트 실행 + guard 규칙 준수 확인         │
│  5. 체크박스 업데이트: TASKS.md [x] 처리             │
│  6. 다음 태스크 있으면 → 1로                        │
└──────────────────────────────────────────────────┘
```

## 단계별 상세

### >>> Stage 1: 태스크 식별

> `ina_report_progress(in_progress="태스크 파악", remaining="{전체 미완료 수}")`

- `TASKS.md` 읽고 미완료(`- [ ]`) 항목 식별
- 관련 플랜 파일(`.claude/plans/*.md`) 읽어서 컨텍스트 확보
- 루프백 재진입 시: `.state/review-issues.md` 존재하면 해당 이슈 우선 처리
- TASKS.md 파싱 실패 시: 사용자에게 경로 확인 요청

### >>> Stage 2: 실행 위임 판단

미완료 태스크들을 분석하여 실행 방식을 결정한다:

```
태스크 분석:

1. 독립성 판단 (LLM)
   - 공유 파일이 있는가? → 순차 실행
   - 다른 디렉토리/모듈인가? → 병렬 후보

2. 태스크 수에 따른 방식 선택
   - 1개 또는 의존성 있는 태스크 → 직접 실행
   - 2-3개 독립 태스크 → Agent 병렬 (subagent)
   - 4개+ 독립 태스크 → Team

3. 도메인 라우팅
   - UI/프론트엔드 태스크 → /ina:design 스킬 참조
   - 외부 리뷰/분석 필요 → codex 위임 (Agent)
```

### >>> Stage 3: 구현

> `ina_report_progress(in_progress="구현: {태스크}", context="위임 방식: {method}")`

#### 직접 실행

- 플랜에 따라 코드 구현
- `/ina:guard` 규칙 준수 (위험 명령 차단, blast radius 체크)

#### 서브에이전트 병렬

독립 태스크 2-3개를 **하나의 메시지에서 Agent를 병렬로** 실행:

```
Agent 1: "TASKS.md의 태스크 A를 구현. 플랜: {plan_excerpt}. 관련 파일: {files}"
Agent 2: "TASKS.md의 태스크 B를 구현. 플랜: {plan_excerpt}. 관련 파일: {files}"
```

**컨텍스트 격리 원칙:** 각 에이전트에 플랜 발췌 + 해당 태스크 + 관련 파일만 전달. 전체 세션 히스토리는 주지 않는다.

#### Team 모드

4개 이상 독립 태스크 시 Team 사용:

1. 태스크를 분석하여 독립 단위로 분해
2. 각 팀원에게 태스크 할당
3. `ina_check_agents`로 다른 에이전트 상태 확인하여 파일 충돌 방지

### >>> Stage 4: 검증

> `ina_report_progress(in_progress="검증: 테스트 실행")`

구현 직후 빠른 검증:

1. 프로젝트 테스트 실행 (CLAUDE.md의 테스트 명령 우선)
2. 실패 시: 분석 → 수정 → 재실행 (최대 3회)
3. 3회 내 미해결 시: `ina_mark_blocked` + 사용자 보고
4. guard 규칙 확인: 변경 파일 수, 테스트 회귀

### >>> Stage 5: 체크박스 업데이트

- TASKS.md에서 완료된 항목을 `- [x]`로 변경
- `.state/progress.md` 업데이트

### >>> Stage 6: 루프

- 다음 미완료 태스크 있으면 → Stage 1로
- 전부 완료:
  1. `.state/review-gate.md` 생성 (리뷰 게이트 활성화):
     ```markdown
     ---
     status: pending
     created_at: "{ISO8601 timestamp}"
     ---
     빌드 완료. `/ina:review` 실행 필요.
     ```
  2. 사용자에게 알림: **"빌드 완료. `/ina:review`로 리뷰를 진행하세요."**
  3. `ina_report_progress(in_progress="BUILD 완료", completed="전체 태스크")`
- 태스크 10개 초과 처리 시: 사용자에게 "계속 진행할까요?" 확인 (무한 실행 방지)

## 오동작 방지

- 각 Stage 진입 시 `>>> Build: {name}` 출력
- guard 규칙 자동 적용 (위험 명령, blast radius, 비밀 정보)
- 태스크 10개 초과 시 사용자 확인
- 테스트 3회 실패 시 즉시 blocked

## 입출력

- **입력**: `TASKS.md` (미완료 `- [ ]` 항목) + `.claude/plans/*.md`
- **출력**: 구현된 코드 + 완료 처리된 `TASKS.md` (`- [x]`)
