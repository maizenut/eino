package skill

import "context"

type CommandToolBuilderConfig struct {
	WorkspaceRoot string
	Shell         CommandShell
	Jobs          CommandBackgroundJobs
	Environments  any
}

type CommandExecuteRequest struct {
	Command         string
	Cwd             string
	EnvironmentName string
	Environment     any
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
