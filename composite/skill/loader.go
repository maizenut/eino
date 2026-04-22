package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	schemad "github.com/cloudwego/eino/schema/declarative"
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
var specFileNames = []string{"SKILL.md", "skill.yaml", "skill.yml", "skill.json"}

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
	spec, err := ParseSkillSpecDocument(data, path)
	if err != nil {
		return nil, fmt.Errorf("decode skill spec %s: %w", path, err)
	}
	return spec, nil
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

type skillDocument struct {
	Info         Info              `json:"info" yaml:"info"`
	Name         string            `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version      string            `json:"version,omitempty" yaml:"version,omitempty"`
	Category     string            `json:"category,omitempty" yaml:"category,omitempty"`
	Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Trigger      *TriggerSpec      `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	Instruction  string            `json:"instruction,omitempty" yaml:"instruction,omitempty"`
	ToolRefs     []schemad.Ref     `json:"tool_refs,omitempty" yaml:"tool_refs,omitempty"`
	CommandTools []CommandToolSpec `json:"command_tools,omitempty" yaml:"command_tools,omitempty"`
	GraphRef     *schemad.Ref      `json:"graph_ref,omitempty" yaml:"graph_ref,omitempty"`
	PromptRef    *schemad.Ref      `json:"prompt_ref,omitempty" yaml:"prompt_ref,omitempty"`
	ModelRef     *schemad.Ref      `json:"model_ref,omitempty" yaml:"model_ref,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ParseSkillSpecDocument decodes a skill document from YAML, JSON, or SKILL.md frontmatter.
func ParseSkillSpecDocument(data []byte, path string) (*SkillSpec, error) {
	doc, markdownInstruction, err := parseSkillDocument(data, path)
	if err != nil {
		return nil, err
	}
	return doc.toSkillSpec(markdownInstruction), nil
}

func parseSkillDocument(data []byte, path string) (*skillDocument, string, error) {
	if strings.EqualFold(filepath.Base(path), "SKILL.md") || strings.EqualFold(filepath.Ext(path), ".md") {
		frontmatter, markdownBody, err := parseMarkdownFrontmatter(data)
		if err != nil {
			return nil, "", err
		}
		var doc skillDocument
		if err := yaml.Unmarshal(frontmatter, &doc); err != nil {
			return nil, "", err
		}
		return &doc, strings.TrimSpace(markdownBody), nil
	}

	var doc skillDocument
	if err := json.Unmarshal(data, &doc); err == nil {
		return &doc, "", nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, "", err
	}
	return &doc, "", nil
}

func parseMarkdownFrontmatter(data []byte) ([]byte, string, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, "", fmt.Errorf("missing SKILL.md frontmatter")
	}
	trimmed := strings.TrimPrefix(content, "---\r\n")
	if trimmed == content {
		trimmed = strings.TrimPrefix(content, "---\n")
	}
	idx := strings.Index(trimmed, "\n---")
	newlineLen := 1
	if idx < 0 {
		idx = strings.Index(trimmed, "\r\n---")
		newlineLen = 2
	}
	if idx < 0 {
		return nil, "", fmt.Errorf("unterminated SKILL.md frontmatter")
	}
	frontmatter := trimmed[:idx]
	bodyStart := idx + newlineLen + len("---")
	body := strings.TrimLeft(trimmed[bodyStart:], "\r\n")
	return []byte(frontmatter), body, nil
}

func (d *skillDocument) toSkillSpec(markdownInstruction string) *SkillSpec {
	info := d.Info
	if info.Name == "" {
		info.Name = d.Name
	}
	if info.Description == "" {
		info.Description = d.Description
	}
	if info.Version == "" {
		info.Version = d.Version
	}
	if info.Category == "" {
		info.Category = d.Category
	}
	if len(info.Tags) == 0 {
		info.Tags = append([]string(nil), d.Tags...)
	}
	instruction := strings.TrimSpace(markdownInstruction)
	if instruction == "" {
		instruction = d.Instruction
	}
	return &SkillSpec{
		Info:         info,
		Trigger:      d.Trigger,
		Instruction:  instruction,
		ToolRefs:     append([]schemad.Ref(nil), d.ToolRefs...),
		CommandTools: append([]CommandToolSpec(nil), d.CommandTools...),
		GraphRef:     d.GraphRef,
		PromptRef:    d.PromptRef,
		ModelRef:     d.ModelRef,
		Metadata:     d.Metadata,
	}
}
