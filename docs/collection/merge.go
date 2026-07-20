package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func main() {
	dir := scriptDir()
	targetFile := filepath.Join(dir, "all.json")

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed reading json files: %v\n", err)
		os.Exit(1)
	}

	filtered := make([]string, 0, len(files))
	targetReal := realPath(targetFile)
	for _, file := range files {
		if realPath(file) == targetReal {
			continue
		}
		filtered = append(filtered, file)
	}
	sort.Strings(filtered)

	mergedItems := make([]map[string]any, 0, len(filtered))
	mergedVariables := []map[string]any{
		{
			"key":   "base_url",
			"value": "http://localhost:8081",
		},
		{
			"key":   "token",
			"value": "",
		},
	}
	seenVariables := map[string]struct{}{
		"base_url": {},
		"token":    {},
	}

	for _, file := range filtered {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skip] cannot read file: %s\n", file)
			continue
		}

		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			fmt.Fprintf(os.Stderr, "[skip] invalid json: %s\n", file)
			continue
		}

		itemsAny, ok := decoded["item"]
		if !ok {
			fmt.Fprintf(os.Stderr, "[skip] no item array: %s\n", file)
			continue
		}

		items, ok := itemsAny.([]any)
		if !ok {
			fmt.Fprintf(os.Stderr, "[skip] no item array: %s\n", file)
			continue
		}

		featureFolderName := deriveFeatureFolderName(file, decoded)
		featureItems := normalizeFeatureItems(items)
		mergedVariables = appendCollectionVariables(mergedVariables, seenVariables, decoded)

		mergedItems = append(mergedItems, map[string]any{
			"name": featureFolderName,
			"item": featureItems,
		})
	}

	allCollection := map[string]any{
		"info": map[string]any{
			"_postman_id": "f69695ee-a016-4edd-ad0a-bf03cdeef374",
			"name":        "Widia Kencana - All",
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
			"description": "Unified Postman collection for Widia Kencana API endpoints.",
		},
		"item":     mergedItems,
		"variable": mergedVariables,
	}

	out, err := json.MarshalIndent(allCollection, "", "    ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed encoding all.json")
		os.Exit(1)
	}

	if err := os.WriteFile(targetFile, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed writing all.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("merged %d file(s) into %s\n", len(filtered), targetFile)
}

func appendCollectionVariables(
	mergedVariables []map[string]any,
	seenVariables map[string]struct{},
	collection map[string]any,
) []map[string]any {
	variables, ok := collection["variable"].([]any)
	if !ok {
		return mergedVariables
	}

	for _, variableAny := range variables {
		variable, ok := variableAny.(map[string]any)
		if !ok {
			continue
		}

		key, ok := variable["key"].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		if _, exists := seenVariables[key]; exists {
			continue
		}

		mergedVariables = append(mergedVariables, variable)
		seenVariables[key] = struct{}{}
	}

	return mergedVariables
}

func deriveFeatureFolderName(file string, collection map[string]any) string {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	name := strings.ReplaceAll(base, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.Title(name) //nolint:staticcheck // keep behavior close to PHP ucwords

	switch base {
	case "analytics":
		return "Analytics (v2)"
	case "analytics-v1", "analytics_v1":
		return "Analytics (v1)"
	case "auth":
		return "Auth"
	}

	infoName := ""
	if info, ok := collection["info"].(map[string]any); ok {
		if val, ok := info["name"].(string); ok {
			infoName = val
		}
	}

	re := regexp.MustCompile(`(?i)v(\d+)`)
	if matches := re.FindStringSubmatch(infoName); len(matches) == 2 {
		return fmt.Sprintf("%s (v%s)", name, matches[1])
	}

	return name
}

func normalizeFeatureItems(items []any) []any {
	// If source collection already wraps all requests in a single folder,
	// unwrap one level to avoid nested folders in all.json.
	if len(items) == 1 {
		if first, ok := items[0].(map[string]any); ok {
			if nested, ok := first["item"].([]any); ok {
				return nested
			}
		}
	}
	return items
}

func scriptDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		cwd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return cwd
	}
	return filepath.Dir(file)
}

func realPath(path string) string {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
