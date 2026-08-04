package agenthooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func mergeConfig(existing []byte, def definition) ([]byte, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := rejectDuplicateJSONKeys(existing); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(existing))
		decoder.UseNumber()
		if err := decoder.Decode(&root); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return nil, err
		}
	}

	hooks, err := objectProperty(root, "hooks", true)
	if err != nil {
		return nil, err
	}
	for _, event := range def.events {
		entries, err := arrayProperty(hooks, event.name)
		if err != nil {
			return nil, err
		}
		entries, err = removeOwnedHandlers(entries, def.hookPath)
		if err != nil {
			return nil, fmt.Errorf("%s hooks: %w", event.name, err)
		}
		entries = append(entries, canonicalEntry(def, event))
		hooks[event.name] = entries
	}

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JSON: %w", err)
	}
	return append(encoded, '\n'), nil
}

func canonicalEntry(def definition, event eventDefinition) map[string]any {
	entry := map[string]any{
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": hookCommand(def, event.lifecycle),
			"timeout": json.Number("6"),
		}},
	}
	if event.matcher != "" {
		entry["matcher"] = event.matcher
	}
	return entry
}

func hookCommand(def definition, lifecycle string) string {
	return "bash " + shellQuote(def.hookPath) + " " + shellQuote(def.engine) + " " +
		shellQuote(lifecycle) + " " + shellQuote(def.orcBinary)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func removeOwnedHandlers(entries []any, hookPath string) ([]any, error) {
	result := make([]any, 0, len(entries))
	for index, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entry %d must be an object", index)
		}
		rawHandlers, exists := entry["hooks"]
		if !exists {
			return nil, fmt.Errorf("entry %d has no hooks array", index)
		}
		handlers, ok := rawHandlers.([]any)
		if !ok {
			return nil, fmt.Errorf("entry %d hooks must be an array", index)
		}
		kept := make([]any, 0, len(handlers))
		for handlerIndex, rawHandler := range handlers {
			handler, ok := rawHandler.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("entry %d handler %d must be an object", index, handlerIndex)
			}
			command, _ := handler["command"].(string)
			if isOwnedCommand(command, hookPath) {
				continue
			}
			kept = append(kept, handler)
		}
		if len(kept) == 0 {
			continue
		}
		entry["hooks"] = kept
		result = append(result, entry)
	}
	return result, nil
}

func isOwnedCommand(command, hookPath string) bool {
	return strings.HasPrefix(command, "bash "+shellQuote(hookPath)+" ")
}

func objectProperty(root map[string]any, name string, create bool) (map[string]any, error) {
	raw, exists := root[name]
	if !exists {
		if !create {
			return nil, nil
		}
		value := map[string]any{}
		root[name] = value
		return value, nil
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", name)
	}
	return value, nil
}

func arrayProperty(root map[string]any, name string) ([]any, error) {
	raw, exists := root[name]
	if !exists {
		return nil, nil
	}
	value, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", name)
	}
	return value, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := validateJSONValue(decoder, "$"); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func validateJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("parse JSON: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("parse JSON: object key at %s is not a string", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON key %q at %s", key, path)
			}
			seen[key] = struct{}{}
			if err := validateJSONValue(decoder, path+"."+key); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := validateJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
			index++
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
	default:
		return fmt.Errorf("parse JSON: unexpected delimiter %q", delim)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	return fmt.Errorf("parse JSON: multiple top-level values")
}
