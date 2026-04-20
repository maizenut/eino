package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SpecLoader loads skill specs from external documents.
type SpecLoader interface {
	LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error)
}

// FileDocumentLoader loads skill specs from the filesystem.
type FileDocumentLoader struct {
	BaseDir string
}

// specFileNames lists candidate filenames when a target resolves to a directory.
var specFileNames = []string{"skill.yaml", "skill.yml", "skill.json"}

// LoadSkillSpec loads a skill spec document from disk.
// When the resolved path is a directory, it probes for skill.yaml / skill.json inside it.
func (l *FileDocumentLoader) LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error) {
	_ = ctx
	path := l.resolvePath(target)
	path = resolveSpecFilePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill spec %s: %w", path, err)
	}

	var spec SkillSpec
	if err := unmarshalSpec(data, &spec); err != nil {
		return nil, fmt.Errorf("decode skill spec %s: %w", path, err)
	}

	return &spec, nil
}

func (l *FileDocumentLoader) resolvePath(target string) string {
	if filepath.IsAbs(target) || l.BaseDir == "" {
		return target
	}
	return filepath.Join(l.BaseDir, target)
}

// resolveSpecFilePath expands a directory path to the first matching spec file inside it.
func resolveSpecFilePath(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return path
	}
	for _, name := range specFileNames {
		candidate := filepath.Join(path, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}

// unmarshalSpec tries JSON first, then falls back to YAML.
func unmarshalSpec(data []byte, spec *SkillSpec) error {
	if err := json.Unmarshal(data, spec); err == nil {
		return nil
	}
	return yaml.Unmarshal(data, spec)
}
