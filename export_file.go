package main

import (
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type SaveTextFileFilter struct {
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
}

type SaveTextFileRequest struct {
	DefaultName string               `json:"defaultName"`
	Content     string               `json:"content"`
	Filters     []SaveTextFileFilter `json:"filters,omitempty"`
}

type SaveTextFileResult struct {
	Path string `json:"path"`
}

func (a *App) SaveTextFile(input SaveTextFileRequest) (SaveTextFileResult, error) {
	filters := normalizeSaveTextFileFilters(input.Filters)
	defaultExt := firstFilterExtension(filters)
	path, err := wailsruntime.SaveFileDialog(a.contextOrBackground(), wailsruntime.SaveDialogOptions{
		Title:                "导出文件",
		DefaultFilename:      safeDefaultFileName(input.DefaultName),
		CanCreateDirectories: true,
		Filters:              filters,
	})
	if err != nil {
		return SaveTextFileResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return SaveTextFileResult{}, errExportCanceled
	}
	return saveTextFileToPath(path, input.Content, defaultExt)
}

var errExportCanceled = exportFileError("已取消导出")

type exportFileError string

func (e exportFileError) Error() string {
	return string(e)
}

func normalizeSaveTextFileFilters(filters []SaveTextFileFilter) []wailsruntime.FileFilter {
	result := make([]wailsruntime.FileFilter, 0, len(filters))
	for _, filter := range filters {
		patterns := make([]string, 0, len(filter.Extensions))
		for _, ext := range filter.Extensions {
			ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
			if ext != "" {
				patterns = append(patterns, "*."+ext)
			}
		}
		if len(patterns) == 0 {
			continue
		}
		name := strings.TrimSpace(filter.Name)
		if name == "" {
			name = "Text Files"
		}
		result = append(result, wailsruntime.FileFilter{
			DisplayName: name,
			Pattern:     strings.Join(patterns, ";"),
		})
	}
	return result
}

func firstFilterExtension(filters []wailsruntime.FileFilter) string {
	if len(filters) == 0 {
		return ""
	}
	for _, part := range strings.Split(filters[0].Pattern, ";") {
		part = strings.TrimSpace(strings.TrimPrefix(part, "*."))
		if part != "" {
			return part
		}
	}
	return ""
}

func safeDefaultFileName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "farm-export.txt"
	}
	return strings.Map(func(ch rune) rune {
		switch ch {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		default:
			return ch
		}
	}, trimmed)
}

func saveTextFileToPath(path string, content string, defaultExt string) (SaveTextFileResult, error) {
	resolved := ensureDefaultExtension(path, defaultExt)
	if parent := filepath.Dir(resolved); parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return SaveTextFileResult{}, err
		}
	}
	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return SaveTextFileResult{}, err
	}
	return SaveTextFileResult{Path: resolved}, nil
}

func ensureDefaultExtension(path string, ext string) string {
	ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
	if ext == "" || filepath.Ext(path) != "" {
		return path
	}
	return path + "." + ext
}
