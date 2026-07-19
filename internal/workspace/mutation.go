package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileEntry struct {
	dest    string
	content string
}

type existingFilePolicy int

const (
	skipExisting existingFilePolicy = iota
	replaceExisting
	rejectExisting
)

type mutationPlanOptions struct {
	existing     existingFilePolicy
	allowUpdates map[string]bool
}

type mutationAction int

const (
	createFile mutationAction = iota
	updateFile
	skipFile
)

func (a mutationAction) String() string {
	switch a {
	case updateFile:
		return "update"
	case skipFile:
		return "skip"
	default:
		return "create"
	}
}

type plannedFile struct {
	entry  fileEntry
	action mutationAction
}

type mutationPlan struct {
	root      string
	files     []plannedFile
	writeFile func(string, []byte, os.FileMode) error
}

type mutationResult struct {
	created int
	updated int
	skipped int
}

func planMutations(root string, entries []fileEntry, opts mutationPlanOptions) (mutationPlan, error) {
	plan := mutationPlan{root: root, files: make([]plannedFile, 0, len(entries))}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		dest, err := mutationDestination(root, entry.dest)
		if err != nil {
			return mutationPlan{}, err
		}
		clean := filepath.ToSlash(filepath.Clean(entry.dest))
		if seen[clean] {
			return mutationPlan{}, fmt.Errorf("duplicate workspace mutation for %s", clean)
		}
		seen[clean] = true
		entry.dest = clean

		action := createFile
		info, err := os.Stat(dest)
		switch {
		case err == nil:
			if info.IsDir() {
				return mutationPlan{}, fmt.Errorf("workspace mutation target is a directory: %s", clean)
			}
			if opts.allowUpdates[clean] || opts.existing == replaceExisting {
				action = updateFile
			} else if opts.existing == skipExisting {
				action = skipFile
			} else {
				return mutationPlan{}, fmt.Errorf("install would overwrite existing file %s", clean)
			}
		case os.IsNotExist(err):
			// createFile
		default:
			return mutationPlan{}, fmt.Errorf("checking %s: %w", clean, err)
		}
		plan.files = append(plan.files, plannedFile{entry: entry, action: action})
	}
	return plan, nil
}

func mutationDestination(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid workspace mutation path %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace mutation path escapes root: %s", rel)
	}
	return filepath.Join(root, clean), nil
}

type fileBackup struct {
	path string
	data []byte
	mode os.FileMode
}

func (p mutationPlan) Apply() (mutationResult, error) {
	writer := p.writeFile
	if writer == nil {
		writer = writeFileAtomic
	}
	var result mutationResult
	var created []string
	var backups []fileBackup
	rollback := func() {
		for i := len(backups) - 1; i >= 0; i-- {
			_ = writeFileAtomic(backups[i].path, backups[i].data, backups[i].mode)
		}
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
	}

	for _, file := range p.files {
		if file.action == skipFile {
			result.skipped++
			continue
		}
		dest, err := mutationDestination(p.root, file.entry.dest)
		if err != nil {
			rollback()
			return mutationResult{}, err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			rollback()
			return mutationResult{}, fmt.Errorf("creating directory for %s: %w", file.entry.dest, err)
		}
		if file.action == updateFile {
			data, err := os.ReadFile(dest)
			if err != nil {
				rollback()
				return mutationResult{}, fmt.Errorf("backing up %s: %w", file.entry.dest, err)
			}
			info, err := os.Stat(dest)
			if err != nil {
				rollback()
				return mutationResult{}, fmt.Errorf("checking %s: %w", file.entry.dest, err)
			}
			backups = append(backups, fileBackup{path: dest, data: data, mode: info.Mode().Perm()})
		}
		if err := writer(dest, []byte(file.entry.content), 0o644); err != nil {
			rollback()
			return mutationResult{}, fmt.Errorf("writing %s: %w", file.entry.dest, err)
		}
		if file.action == updateFile {
			result.updated++
		} else {
			created = append(created, dest)
			result.created++
		}
	}
	return result, nil
}

func writeFileAtomic(dest string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}
