package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func TestRunFlagCompletersUseWorkspaceRepositoriesAndWorkers(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	repos, directive := runRepoCompleter(runCmd, nil, "my")
	if directive != cobra.ShellCompDirectiveNoFileComp || len(repos) != 1 || !strings.HasPrefix(repos[0], "my-app\t") {
		t.Fatalf("repository completions = %v, directive = %v", repos, directive)
	}
	workerIDs, directive := runWorkerCompleter(runCmd, nil, "default:bo")
	if directive != cobra.ShellCompDirectiveNoFileComp || len(workerIDs) != 1 || !strings.HasPrefix(workerIDs[0], "default:bob\t") {
		t.Fatalf("worker completions = %v, directive = %v", workerIDs, directive)
	}
}

func TestFishCompletionGenerated(t *testing.T) {
	var out bytes.Buffer
	if err := rootCmd.GenFishCompletion(&out, true); err != nil {
		t.Fatalf("GenFishCompletion: %v", err)
	}
	completion := out.String()
	for _, want := range []string{"__orc_perform_completion", "__complete"} {
		if !strings.Contains(completion, want) {
			t.Fatalf("Fish completion missing %q", want)
		}
	}
}
