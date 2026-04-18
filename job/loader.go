package job

import "context"

// SpecLoader loads job declarations from external documents.
type SpecLoader interface {
	LoadJobSpec(ctx context.Context, target string) (*JobSpec, error)
}
