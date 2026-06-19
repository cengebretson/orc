package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func runPackList(cmd *cobra.Command, args []string) error {
	packs, err := workspace.ListInstalledPacks(globalWorkspace)
	if err != nil {
		return err
	}
	printPackList(packs)
	return nil
}

func runPackShow(cmd *cobra.Command, args []string) error {
	packs, err := workspace.ListInstalledPacks(globalWorkspace)
	if err != nil {
		return err
	}
	for _, p := range packs {
		if p.Name == args[0] {
			printInstalledPack(p)
			return nil
		}
	}
	return fmt.Errorf("installed pack %q not found", args[0])
}

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

func runPackInstall(cmd *cobra.Command, args []string) error {
	return workspace.InstallPack(workspace.PackInstallOptions{
		Root: globalWorkspace,
		Pack: args[0],
	})
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

func printPackList(packs []workspace.InstalledPack) {
	fmt.Println("Installed packs:")
	if len(packs) == 0 {
		fmt.Println("  none")
		return
	}
	fmt.Println()
	for _, p := range packs {
		status := "inactive"
		if p.Active {
			status = "active"
		}
		fmt.Printf("  %-12s %s\n", p.Name, status)
		fmt.Printf("    path: %s\n", p.Path)
		if p.Install != nil {
			fmt.Printf("    source: %s (%s)\n", p.Install.SourceType, p.Install.SourceRef)
		}
		if p.Inspection != nil {
			if p.Inspection.Manifest.Description != "" {
				fmt.Printf("    description: %s\n", p.Inspection.Manifest.Description)
			}
			printPackInventorySection("workflows", packIDs(p.Inspection.Manifest.Provides.Workflows), p.UsedWorkflows, p.Inspection.Manifest.Aliases.Workflows)
			printPackInventorySection("stages", packIDs(p.Inspection.Manifest.Provides.Stages), nil, p.Inspection.Manifest.Aliases.Stages)
			printPackInventorySection("workers", packIDs(p.Inspection.Manifest.Provides.Workers), nil, p.Inspection.Manifest.Aliases.Workers)
		}
		fmt.Println()
	}
}

func printInstalledPack(p workspace.InstalledPack) {
	fmt.Printf("Pack: %s\n", p.Name)
	fmt.Printf("Installed: %s\n", p.Path)
	status := "inactive"
	if p.Active {
		status = "active"
	}
	fmt.Printf("Status: %s\n\n", status)
	if p.Install != nil {
		fmt.Printf("Source: %s (%s)\n\n", p.Install.SourceType, p.Install.SourceRef)
	}
	if p.Inspection == nil {
		return
	}
	fmt.Printf("Description: %s\n\n", p.Inspection.Manifest.Description)
	printPackResources("Workflows", p.Inspection.Manifest.Provides.Workflows)
	if len(p.UsedWorkflows) > 0 {
		fmt.Printf("Active workflows: %s\n\n", strings.Join(p.UsedWorkflows, ", "))
	} else {
		fmt.Println("Active workflows: none")
		fmt.Println()
	}
	printPackResources("Stages", p.Inspection.Manifest.Provides.Stages)
	printPackResources("Workers", p.Inspection.Manifest.Provides.Workers)
	printPackAliases(p.Inspection.Manifest.Aliases)
}

func printPackInventorySection(title string, ids []string, used []string, aliases map[string]string) {
	if len(ids) == 0 && len(aliases) == 0 {
		return
	}
	fmt.Printf("    %s: %s\n", title, strings.Join(ids, ", "))
	if len(used) > 0 {
		fmt.Printf("    active workflows: %s\n", strings.Join(used, ", "))
	}
	if len(aliases) == 0 {
		return
	}
	keys := make([]string, 0, len(aliases))
	for alias := range aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)
	for _, alias := range keys {
		fmt.Printf("    alias: %s -> %s\n", alias, aliases[alias])
	}
}

func packIDs(resources []workspace.PackResource) []string {
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		ids = append(ids, r.ID)
	}
	return ids
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
