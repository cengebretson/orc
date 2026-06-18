package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func runPackInspect(cmd *cobra.Command, args []string) error {
	report, err := workspace.InspectPack(args[0])
	if err != nil {
		return err
	}

	if packInspectJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		printPackInspection(report)
	}

	if !report.OK() {
		return fmt.Errorf("pack validation failed")
	}
	return nil
}

func printPackInspection(report *workspace.PackInspection) {
	p := report.Manifest
	fmt.Printf("Pack: %s\n", p.Name)
	fmt.Printf("Source: %s\n", report.Source)
	fmt.Printf("Description: %s\n\n", p.Description)

	printPackResources("Workflows", p.Provides.Workflows)
	printPackResources("Stages", p.Provides.Stages)
	printPackResources("Workers", p.Provides.Workers)
	printPackAliases(p.Aliases)

	if report.OK() {
		fmt.Println("OK")
		return
	}

	fmt.Println("Errors:")
	for _, err := range report.Errors {
		fmt.Printf("  - %s\n", err)
	}
}

func printPackResources(title string, resources []workspace.PackResource) {
	fmt.Println(title + ":")
	if len(resources) == 0 {
		fmt.Println("  none")
		fmt.Println()
		return
	}
	for _, r := range resources {
		if r.Description == "" {
			fmt.Printf("  %-24s %s\n", r.ID, r.Path)
		} else {
			fmt.Printf("  %-24s %-28s %s\n", r.ID, r.Path, r.Description)
		}
	}
	fmt.Println()
}

func printPackAliases(aliases workspace.PackAliases) {
	fmt.Println("Aliases:")
	rows := aliasRows("workflow", aliases.Workflows)
	rows = append(rows, aliasRows("stage", aliases.Stages)...)
	rows = append(rows, aliasRows("worker", aliases.Workers)...)
	if len(rows) == 0 {
		fmt.Println("  none")
		fmt.Println()
		return
	}
	sort.Strings(rows)
	for _, row := range rows {
		fmt.Println(row)
	}
	fmt.Println()
}

func aliasRows(kind string, aliases map[string]string) []string {
	rows := make([]string, 0, len(aliases))
	for alias, target := range aliases {
		rows = append(rows, fmt.Sprintf("  %-8s %-16s -> %s", kind, alias, target))
	}
	return rows
}
