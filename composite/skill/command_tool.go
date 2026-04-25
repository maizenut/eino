package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	ftool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	filesystem "github.com/maizenut/mirroru/components/tool/filesystem"
)

var commandTemplatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type CommandToolBuilderConfig struct {
	WorkspaceRoot string
	Shell         filesystem.Shell
	Jobs          filesystem.BackgroundJobs
}

type CommandToolBuilder struct {
	WorkspaceRoot string
	Shell         filesystem.Shell
	Jobs          filesystem.BackgroundJobs
}

func NewCommandToolBuilder(cfg CommandToolBuilderConfig) *CommandToolBuilder {
	return &CommandToolBuilder{
		WorkspaceRoot: strings.TrimSpace(cfg.WorkspaceRoot),
		Shell:         cfg.Shell,
		Jobs:          cfg.Jobs,
	}
}

func (b *CommandToolBuilder) Build(spec CommandToolSpec) (ftool.BaseTool, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("command tool name is required")
	}
	if len(spec.Command.Argv) == 0 && strings.TrimSpace(spec.Command.Command) == "" {
		return nil, fmt.Errorf("command tool %s requires command.argv or command.command", spec.Name)
	}
	if spec.Command.TimeoutMS > 0 {
		return nil, fmt.Errorf("command tool %s timeout_ms is not supported yet", spec.Name)
	}
	if len(spec.Command.Env) > 0 {
		return nil, fmt.Errorf("command tool %s env is not supported yet", spec.Name)
	}

	info, err := buildCommandToolInfo(spec)
	if err != nil {
		return nil, err
	}

	return utils.NewTool(info, func(ctx context.Context, args map[string]any) (string, error) {
		rendered, err := b.render(spec.Command, args)
		if err != nil {
			return "", err
		}
		if spec.Command.Background {
			if b.Jobs == nil {
				return "", fmt.Errorf("background jobs backend is required")
			}
			return b.Jobs.StartJob(ctx, rendered.command, rendered.cwd)
		}
		if b.Shell == nil {
			return "", fmt.Errorf("shell backend is required")
		}
		resp, err := b.Shell.Execute(ctx, &filesystem.ExecuteRequest{
			Command: rendered.command,
			Cwd:     rendered.cwd,
		})
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		sb.WriteString(resp.Output)
		if resp.Truncated {
			sb.WriteString("\n[Output was truncated due to size limits]")
		}
		if resp.ExitCode != nil && *resp.ExitCode != 0 {
			fmt.Fprintf(&sb, "\n[Command failed with exit code %d]", *resp.ExitCode)
		}
		if sb.Len() == 0 {
			return "[Command executed successfully with no output]", nil
		}
		return sb.String(), nil
	}), nil
}

type renderedCommand struct {
	command string
	cwd     string
}

func (b *CommandToolBuilder) render(spec CommandExecutionSpec, args map[string]any) (*renderedCommand, error) {
	var command string
	if len(spec.Argv) > 0 {
		renderedArgv := make([]string, 0, len(spec.Argv))
		for _, arg := range spec.Argv {
			renderedArg, err := renderCommandTemplate(arg, args)
			if err != nil {
				return nil, err
			}
			renderedArgv = append(renderedArgv, renderedArg)
		}
		command = quoteArgv(renderedArgv)
	} else {
		renderedCommand, err := renderCommandTemplate(spec.Command, args)
		if err != nil {
			return nil, err
		}
		command = renderedCommand
	}

	renderedCwd, err := renderCommandTemplate(spec.Cwd, args)
	if err != nil {
		return nil, err
	}
	if renderedCwd != "" && !filepath.IsAbs(renderedCwd) && b.WorkspaceRoot != "" {
		renderedCwd = b.resolveCwd(renderedCwd)
	}

	return &renderedCommand{command: command, cwd: renderedCwd}, nil
}

func (b *CommandToolBuilder) resolveCwd(renderedCwd string) string {
	joined := filepath.Join(b.WorkspaceRoot, renderedCwd)
	if _, err := os.Stat(joined); err == nil {
		return joined
	}
	workspacePrefix := "workspace" + string(filepath.Separator)
	if strings.HasPrefix(renderedCwd, workspacePrefix) && filepath.Base(b.WorkspaceRoot) == "workspace" {
		trimmed := strings.TrimPrefix(renderedCwd, workspacePrefix)
		fallback := filepath.Join(b.WorkspaceRoot, trimmed)
		if _, err := os.Stat(fallback); err == nil {
			return fallback
		}
	}
	return joined
}

func buildCommandToolInfo(spec CommandToolSpec) (*schema.ToolInfo, error) {
	info := &schema.ToolInfo{
		Name: spec.Name,
		Desc: spec.Description,
	}
	if spec.Parameters == nil || len(spec.Parameters.Properties) == 0 {
		return info, nil
	}
	params := make(map[string]*schema.ParameterInfo, len(spec.Parameters.Properties))
	required := make(map[string]struct{}, len(spec.Parameters.Required))
	for _, name := range spec.Parameters.Required {
		required[name] = struct{}{}
	}
	for name, param := range spec.Parameters.Properties {
		parameterInfo, err := buildParameterInfo(param, required, name)
		if err != nil {
			return nil, fmt.Errorf("build parameter %s for command tool %s: %w", name, spec.Name, err)
		}
		params[name] = parameterInfo
	}
	info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	return info, nil
}

func buildParameterInfo(spec CommandParamSchema, required map[string]struct{}, name string) (*schema.ParameterInfo, error) {
	paramType := schema.String
	if spec.Type != "" {
		paramType = schema.DataType(spec.Type)
		switch paramType {
		case schema.Object, schema.Number, schema.Integer, schema.String, schema.Array, schema.Null, schema.Boolean:
		default:
			return nil, fmt.Errorf("unsupported parameter type %q", spec.Type)
		}
	}
	info := &schema.ParameterInfo{
		Type: paramType,
		Desc: spec.Description,
		Enum: append([]string(nil), spec.Enum...),
	}
	if _, ok := required[name]; ok {
		info.Required = true
	}
	if spec.Items != nil {
		item, err := buildParameterInfo(*spec.Items, nil, "")
		if err != nil {
			return nil, err
		}
		info.ElemInfo = item
	}
	if len(spec.Properties) > 0 {
		info.SubParams = make(map[string]*schema.ParameterInfo, len(spec.Properties))
		keys := make([]string, 0, len(spec.Properties))
		for key := range spec.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			subInfo, err := buildParameterInfo(spec.Properties[key], nil, "")
			if err != nil {
				return nil, err
			}
			info.SubParams[key] = subInfo
		}
	}
	return info, nil
}

func renderCommandTemplate(template string, args map[string]any) (string, error) {
	if template == "" {
		return "", nil
	}
	var renderErr error
	rendered := commandTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		if renderErr != nil {
			return match
		}
		parts := commandTemplatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		value, ok := args[parts[1]]
		if !ok {
			return ""
		}
		text, err := stringifyCommandValue(value)
		if err != nil {
			renderErr = fmt.Errorf("render command argument %q: %w", parts[1], err)
			return match
		}
		return text
	})
	if renderErr != nil {
		return "", renderErr
	}
	return rendered, nil
}

func stringifyCommandValue(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", fmt.Errorf("value is null")
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		return fmt.Sprint(typed), nil
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func quoteArgv(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
