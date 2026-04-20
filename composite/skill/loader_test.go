package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileDocumentLoaderYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `info:
  name: yaml-skill
  description: loaded from yaml
  tags:
    - test
trigger:
  strategy: keyword
  keywords:
    - yaml
instruction: |
  This is a yaml-loaded instruction.
tool_refs:
  - kind: interpreter_function
    target: tool.yaml_echo
metadata:
  format: yaml
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := &FileDocumentLoader{BaseDir: dir}
	spec, err := loader.LoadSkillSpec(context.Background(), "skill.yaml")
	if err != nil {
		t.Fatalf("LoadSkillSpec yaml: %v", err)
	}
	if spec.Info.Name != "yaml-skill" {
		t.Fatalf("Info.Name = %q, want yaml-skill", spec.Info.Name)
	}
	if spec.Info.Description != "loaded from yaml" {
		t.Fatalf("Info.Description = %q", spec.Info.Description)
	}
	if len(spec.Info.Tags) != 1 || spec.Info.Tags[0] != "test" {
		t.Fatalf("Info.Tags = %v, want [test]", spec.Info.Tags)
	}
	if spec.Trigger == nil || spec.Trigger.Strategy != "keyword" {
		t.Fatalf("Trigger = %+v", spec.Trigger)
	}
	if len(spec.ToolRefs) != 1 || spec.ToolRefs[0].Target != "tool.yaml_echo" {
		t.Fatalf("ToolRefs = %+v", spec.ToolRefs)
	}
}

func TestFileDocumentLoaderDirectoryDiscovery(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	yamlContent := `info:
  name: dir-skill
  description: discovered in directory
trigger:
  strategy: keyword
  keywords:
    - dir
instruction: directory discovery works
`
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := &FileDocumentLoader{BaseDir: dir}

	// Load by directory name - should discover skill.yaml inside.
	spec, err := loader.LoadSkillSpec(context.Background(), "my-skill")
	if err != nil {
		t.Fatalf("LoadSkillSpec directory: %v", err)
	}
	if spec.Info.Name != "dir-skill" {
		t.Fatalf("Info.Name = %q, want dir-skill", spec.Info.Name)
	}
	if spec.Instruction != "directory discovery works" {
		t.Fatalf("Instruction = %q", spec.Instruction)
	}
}

func TestFileDocumentLoaderJSON(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{"info":{"name":"json-skill"},"instruction":"json works"}`
	if err := os.WriteFile(filepath.Join(dir, "skill.json"), []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := &FileDocumentLoader{BaseDir: dir}
	spec, err := loader.LoadSkillSpec(context.Background(), "skill.json")
	if err != nil {
		t.Fatalf("LoadSkillSpec json: %v", err)
	}
	if spec.Info.Name != "json-skill" {
		t.Fatalf("Info.Name = %q, want json-skill", spec.Info.Name)
	}
}
