package agenthooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		HomeDir:         root,
		CodexHome:       filepath.Join(root, "codex"),
		ClaudeConfigDir: filepath.Join(root, "claude"),
		OrcBinary:       filepath.Join(root, "bin", "orc"),
	}
}

func TestApplyMergesForeignHooksAndIsIdempotent(t *testing.T) {
	opts := testOptions(t)
	for _, dir := range []string{opts.CodexHome, opts.ClaudeConfigDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	codexConfig := []byte("{\n  \"foreign\": 7,\n  \"hooks\": {\n    \"SessionStart\": [{\n      \"matcher\": \"resume\",\n      \"hooks\": [{\"type\": \"command\", \"command\": \"echo keep\"}]\n    }]\n  }\n}\n")
	claudeConfig := []byte("{\"theme\":\"dark\",\"hooks\":{\"Notification\":[{\"matcher\":\"idle_prompt\",\"hooks\":[{\"type\":\"command\",\"command\":\"echo keep\"}]}]}}")
	if err := os.WriteFile(filepath.Join(opts.CodexHome, "hooks.json"), codexConfig, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.ClaudeConfigDir, "settings.json"), claudeConfig, 0o600); err != nil {
		t.Fatal(err)
	}

	plan := BuildPlan(opts)
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	for _, integration := range plan.Integrations {
		hookInfo, err := os.Stat(integration.HookPath)
		if err != nil {
			t.Fatal(err)
		}
		if hookInfo.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %o", integration.HookPath, hookInfo.Mode().Perm())
		}
		config := readJSONObject(t, integration.ConfigPath)
		hooks := objectValue(t, config, "hooks")
		for _, event := range integrationEvents(integration.Engine) {
			if entries, ok := hooks[event].([]any); !ok || len(entries) == 0 {
				t.Fatalf("%s event %s missing: %#v", integration.Engine, event, hooks[event])
			}
		}
	}

	codex := readJSONObject(t, filepath.Join(opts.CodexHome, "hooks.json"))
	if codex["foreign"] != json.Number("7") {
		t.Fatalf("foreign codex setting = %#v", codex["foreign"])
	}
	sessionEntries := arrayValue(t, objectValue(t, codex, "hooks"), "SessionStart")
	if len(sessionEntries) != 2 {
		t.Fatalf("codex SessionStart entries = %#v", sessionEntries)
	}
	claude := readJSONObject(t, filepath.Join(opts.ClaudeConfigDir, "settings.json"))
	if claude["theme"] != "dark" {
		t.Fatalf("foreign claude setting = %#v", claude["theme"])
	}
	notificationEntries := arrayValue(t, objectValue(t, claude, "hooks"), "Notification")
	if len(notificationEntries) != 2 {
		t.Fatalf("claude Notification entries = %#v", notificationEntries)
	}

	second := BuildPlan(opts)
	for _, integration := range second.Integrations {
		if !integration.Ready() {
			t.Fatalf("%s not idempotent: %s", integration.Engine, integration.Summary())
		}
	}
}

func TestApplyRefusesAllWritesWhenConfigIsUnsafe(t *testing.T) {
	opts := testOptions(t)
	if err := os.MkdirAll(opts.CodexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.CodexHome, "hooks.json")
	unsafe := []byte("{\"hooks\":{},\"hooks\":{\"SessionStart\":[]}}")
	if err := os.WriteFile(path, unsafe, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := BuildPlan(opts)
	if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
		t.Fatalf("Apply error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, unsafe) {
		t.Fatal("unsafe config was modified")
	}
	if _, err := os.Stat(filepath.Join(opts.ClaudeConfigDir, "hooks", hookFilename)); !os.IsNotExist(err) {
		t.Fatalf("claude hook should not be partially installed: %v", err)
	}
}

func TestOwnedCommandRequiresCanonicalPrefix(t *testing.T) {
	hookPath := "/config/hooks/orc-agent-state.sh"
	if !isOwnedCommand("bash '/config/hooks/orc-agent-state.sh' 'codex' 'idle' '/bin/orc'", hookPath) {
		t.Fatal("canonical Orc command was not recognized")
	}
	if isOwnedCommand("echo '/config/hooks/orc-agent-state.sh'", hookPath) {
		t.Fatal("foreign command merely mentioning the hook path was treated as owned")
	}
}

func TestInstalledHookForwardsPayloadToOrcWithoutPython(t *testing.T) {
	opts := testOptions(t)
	output := filepath.Join(t.TempDir(), "args")
	fakeOrc := opts.OrcBinary
	if err := os.MkdirAll(filepath.Dir(fakeOrc), 0o700); err != nil {
		t.Fatal(err)
	}
	fake := "#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" > \"$FAKE_ORC_OUTPUT\"\ncat > \"${FAKE_ORC_OUTPUT}.stdin\"\n"
	if err := os.WriteFile(fakeOrc, []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	plan := BuildPlan(opts)
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	hookPath := plan.Integrations[0].HookPath
	payload := "{\"session_id\":\"provider-1\",\"turn_id\":\"turn-1\",\"hook_event_name\":\"PermissionRequest\",\"tool_use_id\":\"tool-1\"}"
	runHook(t, hookPath, fakeOrc, output, payload)
	args, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got := string(args)
	for _, want := range []string{
		"agent-event\n", "--hook-input\n", "--engine\n", "codex\n", "--state\n", "blocked\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hook args missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"--agent-id", "--instance", "--provider-session", "--event-id", "python3"} {
		if strings.Contains(got, unwanted) || strings.Contains(string(hookScript), unwanted) {
			t.Fatalf("hook still contains obsolete dependency or normalized flag %q", unwanted)
		}
	}
	forwarded, err := os.ReadFile(output + ".stdin")
	if err != nil {
		t.Fatal(err)
	}
	if string(forwarded) != payload {
		t.Fatalf("forwarded payload = %q, want %q", forwarded, payload)
	}
}

func TestForeignHooksFindsOnlyClaimedEventsFromOtherTools(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "hooks", "orc-agent-state.sh")
	config := `{"hooks":{
      "Stop":[{"hooks":[
        {"command":"\"$HOME/hooks/dispatch.sh\" agent-turn-stop"},
        {"command":"` + strings.ReplaceAll(hookPath, `\`, `\\`) + ` codex idle"}
      ]}],
      "PreToolUse":[{"hooks":[{"command":"some-unclaimed-event-hook"}]}]
    }}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	got := ForeignHooks(Integration{
		ConfigPath: configPath,
		HookPath:   hookPath,
		Events:     []string{"Stop", "UserPromptSubmit"},
	})

	if len(got) != 1 {
		t.Fatalf("ForeignHooks = %#v, want exactly the one foreign Stop hook", got)
	}
	if !strings.Contains(got[0], "agent-turn-stop") {
		t.Errorf("ForeignHooks = %q, want the dispatcher command", got[0])
	}
	// Orc's own hook must not count as a conflict with itself, and an event
	// Orc does not claim is none of its business.
	for _, entry := range got {
		if strings.Contains(entry, hookPath) || strings.Contains(entry, "unclaimed") {
			t.Errorf("ForeignHooks included %q", entry)
		}
	}
}

func TestForeignHooksIgnoresMissingOrUnreadableConfig(t *testing.T) {
	missing := ForeignHooks(Integration{
		ConfigPath: filepath.Join(t.TempDir(), "absent.json"),
		Events:     []string{"Stop"},
	})
	if missing != nil {
		t.Errorf("missing config = %#v, want nil", missing)
	}

	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPath, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ForeignHooks(Integration{ConfigPath: badPath, Events: []string{"Stop"}}); got != nil {
		t.Errorf("unparseable config = %#v, want nil", got)
	}
}

func TestDetectVersionUsesFirstNonEmptyLine(t *testing.T) {
	// Spawning a just-written script while the rest of the suite compiles and
	// runs in parallel can exceed the production default, which failed this
	// test on machine load rather than on behavior. The probe's bound is not
	// what is under test here.
	previous := detectVersionTimeout
	detectVersionTimeout = 30 * time.Second
	t.Cleanup(func() { detectVersionTimeout = previous })

	binary := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(binary, []byte("#!/usr/bin/env bash\nprintf '\\nagent 1.2.3\\nextra\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := DetectVersion(binary)
	if err != nil {
		t.Fatal(err)
	}
	if got != "agent 1.2.3" {
		t.Fatalf("version = %q", got)
	}
}

func runHook(t *testing.T, hookPath, fakeOrc, output, payload string) {
	t.Helper()
	cmd := exec.Command("bash", hookPath, "codex", "blocked", fakeOrc)
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = append(os.Environ(),
		"ORC_AGENT_ID=agent-1",
		"ORC_AGENT_INSTANCE=instance-1",
		"TMUX_PANE=%7",
		"FAKE_ORC_OUTPUT="+output,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
}

func readJSONObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func integrationEvents(engine string) []string {
	if engine == "codex" {
		return []string{"SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop"}
	}
	return []string{"SessionStart", "UserPromptSubmit", "PermissionRequest", "Notification", "PostToolUse", "Stop", "StopFailure"}
}

func objectValue(t *testing.T, root map[string]any, name string) map[string]any {
	t.Helper()
	value, ok := root[name].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", name, root[name])
	}
	return value
}

func arrayValue(t *testing.T, root map[string]any, name string) []any {
	t.Helper()
	value, ok := root[name].([]any)
	if !ok {
		t.Fatalf("%s is not an array: %#v", name, root[name])
	}
	return value
}
