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
)

var commandTemplatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type CommandToolBuilderConfig struct {
	WorkspaceRoot string
	Shell         CommandShell
	Jobs          CommandBackgroundJobs
}

type CommandExecuteRequest struct {
	Command string
	Cwd     string
}

type CommandExecuteResponse struct {
	Output    string
	ExitCode  *int
	Truncated bool
}

type CommandShell interface {
	Execute(ctx context.Context, req *CommandExecuteRequest) (*CommandExecuteResponse, error)
}

type CommandBackgroundJobs interface {
	StartJob(ctx context.Context, command string, cwd string) (string, error)
	GetJobOutput(ctx context.Context, jobID string) (string, error)
	KillJob(ctx context.Context, jobID string) error
	ListJobs(ctx context.Context) ([]CommandJobInfo, error)
}

type CommandJobInfo struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

type CommandToolBuilder struct {
	WorkspaceRoot string
	Shell         CommandShell
	Jobs          CommandBackgroundJobs
}

func NewCommandToolBuilder(cfg CommandToolBuilderConfig) *CommandToolBuilder {
	return &CommandToolBuilder{
		WorkspaceRoot: strings.TrimSpace(cfg.WorkspaceRoot),
		Shell:         cfg.Shell,
		Jobs:          cfg.Jobs,
	}
}

func (b *CommandToolBuilder) Build(spec CommandToolSpec) (ftool.BaseTool, error) {
	return b.BuildWithProfile(spec, CommandProfileSpec{})
}

func (b *CommandToolBuilder) BuildWithProfile(spec CommandToolSpec, profile CommandProfileSpec) (ftool.BaseTool, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("command tool name is required")
	}
	kind := normalizeCommandToolKind(spec.Kind, spec.Command.Background)
	if commandToolKindNeedsCommand(kind) && len(spec.Command.Argv) == 0 && strings.TrimSpace(spec.Command.Command) == "" && strings.TrimSpace(profile.Executable) == "" {
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
		switch kind {
		case "filesystem.get_background_job_output":
			return b.getBackgroundJobOutput(ctx, args)
		case "filesystem.kill_background_job":
			return b.killBackgroundJob(ctx, args)
		case "filesystem.list_background_jobs":
			return b.listBackgroundJobs(ctx)
		}

		rendered, err := b.render(spec.Command, profile, args)
		if err != nil {
			return "", err
		}
		if kind == "filesystem.start_background_job" {
			if b.Jobs == nil {
				return "", fmt.Errorf("background jobs backend is required")
			}
			return b.Jobs.StartJob(ctx, rendered.command, rendered.cwd)
		}
		if b.Shell == nil {
			return "", fmt.Errorf("shell backend is required")
		}
		resp, err := b.Shell.Execute(ctx, &CommandExecuteRequest{
			Command: rendered.command,
			Cwd:     rendered.cwd,
		})
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", fmt.Errorf("shell backend returned nil response")
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

func normalizeCommandToolKind(kind string, background bool) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		if background {
			return "filesystem.start_background_job"
		}
		return "filesystem.execute"
	}
	switch trimmed {
	case "execute":
		return "filesystem.execute"
	case "start_background", "start_background_job":
		return "filesystem.start_background_job"
	case "get_background", "get_background_job_output":
		return "filesystem.get_background_job_output"
	case "kill_background", "kill_background_job":
		return "filesystem.kill_background_job"
	case "list_background", "list_background_jobs":
		return "filesystem.list_background_jobs"
	default:
		return trimmed
	}
}

func commandToolKindNeedsCommand(kind string) bool {
	switch kind {
	case "filesystem.execute", "filesystem.start_background_job":
		return true
	case "filesystem.get_background_job_output", "filesystem.kill_background_job", "filesystem.list_background_jobs":
		return false
	default:
		return true
	}
}

type renderedCommand struct {
	command string
	cwd     string
}

func (b *CommandToolBuilder) render(spec CommandExecutionSpec, profile CommandProfileSpec, args map[string]any) (*renderedCommand, error) {
	var command string
	if len(spec.Argv) > 0 {
		renderedArgv := make([]string, 0, len(profile.DefaultArgs)+len(spec.Argv)+1)
		if executable := strings.TrimSpace(profile.Executable); executable != "" {
			renderedArgv = append(renderedArgv, executable)
		}
		renderedArgv = append(renderedArgv, profile.DefaultArgs...)
		for _, arg := range spec.Argv {
			renderedArgs, err := renderCommandArgvTemplate(arg, args)
			if err != nil {
				return nil, err
			}
			renderedArgv = append(renderedArgv, renderedArgs...)
		}
		command = quoteArgv(renderedArgv)
	} else {
		if strings.TrimSpace(spec.Command) != "" {
			renderedCommand, err := renderCommandTemplate(spec.Command, args)
			if err != nil {
				return nil, err
			}
			command = renderedCommand
		} else {
			renderedArgv := make([]string, 0, len(profile.DefaultArgs)+1)
			if executable := strings.TrimSpace(profile.Executable); executable != "" {
				renderedArgv = append(renderedArgv, executable)
			}
			renderedArgv = append(renderedArgv, profile.DefaultArgs...)
			command = quoteArgv(renderedArgv)
		}
	}

	cwdTemplate := spec.Cwd
	if strings.TrimSpace(cwdTemplate) == "" {
		cwdTemplate = profile.Cwd
	}
	renderedCwd, err := renderCommandTemplate(cwdTemplate, args)
	if err != nil {
		return nil, err
	}
	if renderedCwd != "" && !filepath.IsAbs(renderedCwd) && b.WorkspaceRoot != "" {
		renderedCwd = b.resolveCwd(renderedCwd)
	}

	return &renderedCommand{command: command, cwd: renderedCwd}, nil
}

func (b *CommandToolBuilder) getBackgroundJobOutput(ctx context.Context, args map[string]any) (string, error) {
	if b.Jobs == nil {
		return "", fmt.Errorf("background jobs backend is required")
	}
	jobID := commandJobID(args)
	if jobID == "" {
		return "", fmt.Errorf("job_id is required")
	}
	return b.Jobs.GetJobOutput(ctx, jobID)
}

func (b *CommandToolBuilder) killBackgroundJob(ctx context.Context, args map[string]any) (string, error) {
	if b.Jobs == nil {
		return "", fmt.Errorf("background jobs backend is required")
	}
	jobID := commandJobID(args)
	if jobID == "" {
		return "", fmt.Errorf("job_id is required")
	}
	if err := b.Jobs.KillJob(ctx, jobID); err != nil {
		return "", err
	}
	return "OK", nil
}

func (b *CommandToolBuilder) listBackgroundJobs(ctx context.Context) (string, error) {
	if b.Jobs == nil {
		return "", fmt.Errorf("background jobs backend is required")
	}
	jobs, err := b.Jobs.ListJobs(ctx)
	if err != nil {
		return "", err
	}
	if len(jobs) == 0 {
		return "No jobs found", nil
	}
	var sb strings.Builder
	for _, job := range jobs {
		fmt.Fprintf(&sb, "%s\t%s\t%s\n", job.ID, job.Status, job.Command)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func commandJobID(args map[string]any) string {
	jobID := strings.TrimSpace(fmt.Sprint(args["job_id"]))
	if jobID == "<nil>" {
		return ""
	}
	return jobID
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

func renderCommandArgvTemplate(template string, args map[string]any) ([]string, error) {
	if template == "" {
		return []string{""}, nil
	}
	if key, ok := wholeCommandTemplateKey(template); ok {
		value, exists := args[key]
		if !exists {
			return []string{""}, nil
		}
		values, err := stringifyCommandArgvValue(value)
		if err != nil {
			return nil, fmt.Errorf("render command argument %q: %w", key, err)
		}
		return values, nil
	}
	rendered, err := renderCommandTemplate(template, args)
	if err != nil {
		return nil, err
	}
	return []string{rendered}, nil
}

func wholeCommandTemplateKey(template string) (string, bool) {
	matches := commandTemplatePattern.FindStringSubmatch(template)
	if len(matches) != 2 || strings.TrimSpace(template) != matches[0] {
		return "", false
	}
	return matches[1], true
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

func stringifyCommandArgvValue(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, err := stringifyCommandValue(item)
			if err != nil {
				return nil, err
			}
			out = append(out, text)
		}
		return out, nil
	default:
		text, err := stringifyCommandValue(value)
		if err != nil {
			return nil, err
		}
		return []string{text}, nil
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
