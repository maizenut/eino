package mcp

import (
	"context"
	"fmt"
)

// Service wires MCP loading, registration, and runtime access.
type Service struct {
	Loader   SpecLoader
	Registry Registry
}

// NewService creates an MCP service from explicit dependencies.
func NewService(loader SpecLoader, registry Registry) *Service {
	return &Service{
		Loader:   loader,
		Registry: registry,
	}
}

// Load reads an MCP server spec from an external target.
func (s *Service) Load(ctx context.Context, target string) (*ServerSpec, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("mcp loader is required")
	}
	if target == "" {
		return nil, fmt.Errorf("mcp target is required")
	}
	return s.Loader.LoadServerSpec(ctx, target)
}

// Register stores a server spec in the registry.
func (s *Service) Register(ctx context.Context, spec *ServerSpec, opts ...Option) (string, error) {
	if s == nil || s.Registry == nil {
		return "", fmt.Errorf("mcp registry is required")
	}
	return s.Registry.Register(ctx, spec, opts...)
}

// Unregister removes a server spec from the registry.
func (s *Service) Unregister(ctx context.Context, serverID string) error {
	if s == nil || s.Registry == nil {
		return fmt.Errorf("mcp registry is required")
	}
	if serverID == "" {
		return fmt.Errorf("mcp server id is required")
	}
	return s.Registry.Unregister(ctx, serverID)
}

// LoadAndRegister loads a server spec and stores it in the registry.
func (s *Service) LoadAndRegister(ctx context.Context, target string, opts ...Option) (*ServerSpec, string, error) {
	spec, err := s.Load(ctx, target)
	if err != nil {
		return nil, "", err
	}
	serverID, err := s.Register(ctx, spec, opts...)
	if err != nil {
		return nil, "", err
	}
	return spec, serverID, nil
}

// GetSpec returns a registered server spec by id.
func (s *Service) GetSpec(ctx context.Context, serverID string) (*ServerSpec, bool, error) {
	if s == nil || s.Registry == nil {
		return nil, false, fmt.Errorf("mcp registry is required")
	}
	if serverID == "" {
		return nil, false, fmt.Errorf("mcp server id is required")
	}
	spec, ok := s.Registry.GetSpec(ctx, serverID)
	return spec, ok, nil
}

// List returns all registered server specs.
func (s *Service) List(ctx context.Context) ([]ServerSpec, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("mcp registry is required")
	}
	return s.Registry.List(ctx)
}

// GetClient returns a connected MCP client by server id.
func (s *Service) GetClient(ctx context.Context, serverID string) (Client, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("mcp registry is required")
	}
	if serverID == "" {
		return nil, fmt.Errorf("mcp server id is required")
	}
	return s.Registry.GetClient(ctx, serverID)
}

// GetRuntimeView returns the assembled runtime view by server id.
func (s *Service) GetRuntimeView(ctx context.Context, serverID string) (*RuntimeView, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("mcp registry is required")
	}
	if serverID == "" {
		return nil, fmt.Errorf("mcp server id is required")
	}
	return s.Registry.GetRuntimeView(ctx, serverID)
}
