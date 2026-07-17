package tmux

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWriteScriptCleansUpAfterCommandFailure(t *testing.T) {
	script, err := writeScript(t.TempDir(), []string{"false"})
	if err != nil {
		t.Fatalf("writeScript: %v", err)
	}

	if err := exec.Command("bash", script).Run(); err == nil {
		t.Fatal("expected script command to fail")
	}
	if _, err := os.Stat(script); !os.IsNotExist(err) {
		t.Fatalf("script still exists after failed command: %v", err)
	}
}

func TestParseDetailedPanesPreservesEmptyAttention(t *testing.T) {
	row := strings.Join([]string{
		"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
		"ORC-1", "develop", "default:bob", "codex", "codex", "provider-1", "/feature", "",
	}, "\t") + "\n"

	got := parseDetailedPanes([]byte(row))
	if len(got) != 1 {
		t.Fatalf("panes = %#v", got)
	}
	if got[0].ID != "%1" || got[0].ProviderSessionID != "provider-1" || got[0].Attention != "" {
		t.Fatalf("pane = %#v", got[0])
	}
}

func TestWriteScriptQuotesArguments(t *testing.T) {
	runDir := t.TempDir()
	script, err := writeScript(runDir, []string{"printf", "%s", "value with 'quotes'"})
	if err != nil {
		t.Fatalf("writeScript: %v", err)
	}
	defer os.Remove(script) //nolint:errcheck

	content, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "trap 'rm -f ") {
		t.Fatalf("script missing cleanup trap:\n%s", text)
	}
	if !strings.Contains(text, "cd "+shellQuote(runDir)+" || exit 1") {
		t.Fatalf("script missing guarded cd into run dir:\n%s", text)
	}
	if !strings.Contains(text, shellQuote("value with 'quotes'")) {
		t.Fatalf("script missing quoted argument:\n%s", text)
	}
	if !strings.Contains(text, "trap - EXIT\nexec printf") {
		t.Fatalf("script must replace itself with the provider command:\n%s", text)
	}
}

func TestWriteScriptRejectsEmptyCommand(t *testing.T) {
	if _, err := writeScript(t.TempDir(), nil); err == nil {
		t.Fatal("writeScript should reject an empty command")
	}
}

func TestSetSessionEnvironmentRejectsUnsafeName(t *testing.T) {
	for _, name := range []string{"", "-g", "1BAD", "BAD=VALUE", "BAD NAME"} {
		if err := SetSessionEnvironment("unused", name, "value"); err == nil {
			t.Errorf("SetSessionEnvironment should reject %q", name)
		}
		if _, err := SessionEnvironment("unused", name); err == nil {
			t.Errorf("SessionEnvironment should reject %q", name)
		}
	}
}

func TestBuildWatchCommandQuotesArguments(t *testing.T) {
	cmd := buildWatchCommand(WatchToggleOptions{
		ExecPath:  "/tmp/orc bin/orc",
		Root:      "/tmp/work space",
		Ticket:    "PROJ-123",
		Interval:  "2s",
		Wide:      true,
		View:      "pet",
		PetLayout: "column",
	})

	for _, want := range []string{
		shellQuote("/tmp/orc bin/orc"),
		"--workspace " + shellQuote("/tmp/work space"),
		"watch PROJ-123",
		"--interval 2s",
		"--wide",
		"--view pet",
		"--pet-layout column",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("buildWatchCommand() missing %q in %q", want, cmd)
		}
	}
	if strings.Contains(cmd, "--tmux-toggle") {
		t.Fatalf("buildWatchCommand() must not recurse with --tmux-toggle: %q", cmd)
	}
}

func TestBuildWatchCommandOmitsDefaultPetOptions(t *testing.T) {
	cmd := buildWatchCommand(WatchToggleOptions{
		ExecPath:  "orc",
		View:      "rail",
		PetLayout: "responsive",
	})
	for _, flag := range []string{"--view", "--pet-layout", "--tmux-toggle"} {
		if strings.Contains(cmd, flag) {
			t.Fatalf("buildWatchCommand() included default-only flag %q in %q", flag, cmd)
		}
	}
}

func TestWatchSplitFlag(t *testing.T) {
	tests := []struct {
		layout string
		want   string
	}{
		{"", "-h"},
		{"right", "-h"},
		{"bottom", "-v"},
	}

	for _, tt := range tests {
		t.Run(tt.layout, func(t *testing.T) {
			got, err := watchSplitFlag(tt.layout)
			if err != nil {
				t.Fatalf("watchSplitFlag(%q): %v", tt.layout, err)
			}
			if got != tt.want {
				t.Fatalf("watchSplitFlag(%q) = %q, want %q", tt.layout, got, tt.want)
			}
		})
	}

	if _, err := watchSplitFlag("left"); err == nil {
		t.Fatal("watchSplitFlag(left) should fail")
	}
}
