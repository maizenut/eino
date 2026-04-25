package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

var (
	skillCodeSpanPattern    = regexp.MustCompile("`([^`]+)`")
	skillTokenPattern       = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._/-]*`)
	skillMakeCommandPattern = regexp.MustCompile(`(?i)\bmake\s+[A-Za-z0-9._/-]+\b`)
)

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
	trigger := d.Trigger
	if trigger == nil && markdownInstruction != "" {
		trigger = inferMarkdownTrigger(info, instruction)
	}
	metadata := cloneSkillMetadata(d.Metadata)
	if markdownInstruction != "" {
		metadata = inferMarkdownMetadata(metadata, instruction)
	}
	return &SkillSpec{
		Info:         info,
		Trigger:      trigger,
		Instruction:  instruction,
		ToolRefs:     append([]schemad.Ref(nil), d.ToolRefs...),
		CommandTools: append([]CommandToolSpec(nil), d.CommandTools...),
		GraphRef:     d.GraphRef,
		PromptRef:    d.PromptRef,
		ModelRef:     d.ModelRef,
		Metadata:     metadata,
	}
}

func inferMarkdownTrigger(info Info, instruction string) *TriggerSpec {
	patterns := make([]string, 0)
	seen := make(map[string]struct{})
	addPattern := func(pattern string) {
		if pattern == "" {
			return
		}
		if _, ok := seen[pattern]; ok {
			return
		}
		seen[pattern] = struct{}{}
		patterns = append(patterns, pattern)
	}

	joined := strings.ToLower(strings.Join([]string{info.Name, info.Description, instruction}, "\n"))
	if strings.Contains(joined, "smoke test") {
		addPattern(`(?i)smoke\s+test`)
	}

	for _, source := range []string{info.Name, info.Description, instruction} {
		for _, match := range skillCodeSpanPattern.FindAllStringSubmatch(source, -1) {
			for _, token := range skillTokenPattern.FindAllString(match[1], -1) {
				addSkillTokenPattern(token, addPattern)
			}
		}
		for _, token := range skillTokenPattern.FindAllString(source, -1) {
			addSkillTokenPattern(token, addPattern)
		}
	}

	if len(patterns) == 0 {
		return nil
	}
	return &TriggerSpec{Strategy: TriggerStrategyPattern, Patterns: patterns}
}

func addSkillTokenPattern(token string, add func(string)) {
	token = strings.TrimSpace(token)
	if len(token) < 4 {
		return
	}
	hasLetter := false
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return
	}
	add(`(?i)` + regexp.QuoteMeta(token))
}

func cloneSkillMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func inferMarkdownMetadata(metadata map[string]any, instruction string) map[string]any {
	if metadata != nil {
		if _, ok := metadata["test_command"]; ok {
			return metadata
		}
	}
	match := skillMakeCommandPattern.FindString(instruction)
	if match == "" {
		return metadata
	}
	if metadata == nil {
		metadata = make(map[string]any, 1)
	}
	metadata["test_command"] = strings.ToLower(strings.TrimSpace(match))
	return metadata
}
