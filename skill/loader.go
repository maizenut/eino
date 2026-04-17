package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SpecLoader loads skill specs from external documents.
type SpecLoader interface {
	LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error)
}

// FileDocumentLoader loads skill specs from the filesystem.
type FileDocumentLoader struct {
	BaseDir string
}

// LoadSkillSpec loads a skill spec document from disk.
func (l *FileDocumentLoader) LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error) {
	_ = ctx
	path := l.resolvePath(target)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill spec %s: %w", path, err)
	}

	var spec SkillSpec
	if err := json.Unmarshal(data, &spec); err != nil {
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
