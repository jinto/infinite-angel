---
name: guard
description: 무인 실행 안전장치 — 위험 명령 차단, blast radius 체크
argument-hint: (자동 적용 — 직접 호출 불필요)
user-invocable: false
---

# Guard

무인 실행 시 안전장치. autopilot이나 build가 실행되는 동안 자동으로 적용된다.

이 스킬은 직접 호출하지 않는다. 다른 스킬(autopilot, build)이 실행 중 참조하는 규칙 세트이다.

## 규칙

### 1. 위험 명령 차단

다음 패턴이 감지되면 실행 전 경고하고 사용자 확인을 요청한다:

- `rm -rf` / `rmdir` (재귀 삭제)
- `DROP TABLE` / `DROP DATABASE`
- `git push --force` / `git reset --hard`
- `kubectl delete`
- 프로덕션 환경 변수 수정 (`.env.production`, `prod` 포함 config)
- `chmod 777` / `chmod -R`

**비인터랙티브 모드에서:** 위험 명령을 차단하고 `ina_mark_blocked(reason="safety: {command}")` 호출.

### 2. Blast Radius 체크

구현 중 변경 파일 수를 모니터링:

| 변경 파일 수 | 동작 |
|-------------|------|
| ≤ 10 | 정상 진행 |
| 11-30 | 경고 출력 (계속 진행) |
| 31+ | 사용자 확인 요청. 비인터랙티브 시 `ina_mark_blocked` |

확인 방법: `git diff --name-only | wc -l`

### 3. 테스트 회귀 보호

구현 전후 테스트 결과 비교:
- 구현 전 통과했던 테스트가 구현 후 실패하면 경고
- 3개 이상 기존 테스트가 깨지면 사용자 확인 요청

### 4. 비밀 정보 보호

다음 파일을 수정하려 할 때 경고:
- `.env`, `.env.*`
- `credentials.*`, `secrets.*`
- `*.pem`, `*.key`
- SSH/API 키를 포함하는 것으로 보이는 파일

## 적용 방식

이 규칙은 **다른 스킬의 SKILL.md에서 참조**된다:

```
구현 시 /ina:guard 규칙을 준수한다:
- 위험 명령 실행 전 확인
- 변경 파일 31개 초과 시 중단
- 비밀 정보 파일 수정 금지
```

## 입출력

- **입력**: 없음 (규칙 참조용)
- **출력**: 없음
