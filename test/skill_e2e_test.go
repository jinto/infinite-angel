package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Tier 2: 유료, 스킬 변경 시 — E2E 스모크 테스트
// claude -p로 실제 세션을 돌려서 올바른 스킬이 호출되는지 확인
//
// 실행: INFA_E2E=1 go test ./test/ -run TestSkillRouting -v -timeout 600s

type scenario struct {
	Name        string  `json:"name"`
	Input       string  `json:"input"`
	ExpectSkill string  `json:"expect_skill"`
	ExpectMode  *string `json:"expect_mode"`
	Description string  `json:"description"`
}

func TestSkillRouting(t *testing.T) {
	if os.Getenv("INFA_E2E") == "" {
		t.Skip("E2E tests disabled. Set INFA_E2E=1 to run (costs API credits).")
	}

	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found in PATH")
	}

	data, err := os.ReadFile("scenarios.json")
	if err != nil {
		t.Fatalf("cannot read scenarios.json: %v", err)
	}

	var scenarios []scenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		t.Fatalf("parse scenarios.json: %v", err)
	}

	for _, sc := range scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			result := runClaudeSession(t, sc)
			checkSkillInvocation(t, sc, result)
		})
	}
}

func runClaudeSession(t *testing.T, sc scenario) string {
	t.Helper()

	prompt := fmt.Sprintf(`You are testing the ina plugin's skill routing.
Given this user request: %q

Which ina skill should be invoked? Answer with ONLY the skill name in this format:
SKILL: ina:<name>
MODE: <mode or none>

Do not explain. Just output SKILL and MODE lines.`, sc.Input)

	ctx_timeout := 30 * time.Second
	cmd := exec.Command("claude", "-p", "--output-format", "text", prompt)
	cmd.Dir = ".."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start claude: %v", err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("stderr: %s", stderr.String())
			t.Fatalf("claude exited with error: %v", err)
		}
	case <-time.After(ctx_timeout):
		cmd.Process.Kill()
		t.Fatalf("claude timed out after %v", ctx_timeout)
	}

	return stdout.String()
}

func checkSkillInvocation(t *testing.T, sc scenario, output string) {
	t.Helper()

	lines := strings.Split(output, "\n")
	var skillLine, modeLine string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SKILL:") {
			skillLine = strings.TrimSpace(strings.TrimPrefix(line, "SKILL:"))
		}
		if strings.HasPrefix(line, "MODE:") {
			modeLine = strings.TrimSpace(strings.TrimPrefix(line, "MODE:"))
		}
	}

	if skillLine == "" {
		t.Errorf("no SKILL line in output.\nInput: %s\nOutput: %s", sc.Input, output)
		return
	}

	if skillLine != sc.ExpectSkill {
		t.Errorf("wrong skill.\nInput: %s\nExpected: %s\nGot: %s", sc.Input, sc.ExpectSkill, skillLine)
	}

	if sc.ExpectMode != nil && modeLine != *sc.ExpectMode {
		t.Errorf("wrong mode.\nInput: %s\nExpected: %s\nGot: %s", sc.Input, *sc.ExpectMode, modeLine)
	}
}
