package channel

import (
	"context"
	"fmt"
)

// Service wires channel loading, registration, and runtime access.
type Service struct {
	Loader   SpecLoader
	Registry Registry
}

// NewService creates a channel service from explicit dependencies.
func NewService(loader SpecLoader, registry Registry) *Service {
	return &Service{Loader: loader, Registry: registry}
}

// Load reads a channel spec from an external target.
func (s *Service) Load(ctx context.Context, target string) (*ChannelSpec, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("channel loader is required")
	}
	if target == "" {
		return nil, fmt.Errorf("channel target is required")
	}
	return s.Loader.LoadChannelSpec(ctx, target)
}

// Register stores a channel spec in the registry and assembles its runtime.
func (s *Service) Register(ctx context.Context, spec *ChannelSpec, opts ...Option) (string, error) {
	if s == nil || s.Registry == nil {
		return "", fmt.Errorf("channel registry is required")
	}
	return s.Registry.Register(ctx, spec, opts...)
}

// Unregister removes a channel from the registry and closes its connection.
func (s *Service) Unregister(ctx context.Context, channelID string) error {
	if s == nil || s.Registry == nil {
		return fmt.Errorf("channel registry is required")
	}
	if channelID == "" {
		return fmt.Errorf("channel id is required")
	}
	return s.Registry.Unregister(ctx, channelID)
}

// LoadAndRegister loads a channel spec and registers it in a single call.
func (s *Service) LoadAndRegister(ctx context.Context, target string, opts ...Option) (*ChannelSpec, string, error) {
	spec, err := s.Load(ctx, target)
	if err != nil {
		return nil, "", err
	}
	channelID, err := s.Register(ctx, spec, opts...)
	if err != nil {
		return nil, "", err
	}
	return spec, channelID, nil
}

// GetSpec returns a registered channel spec by id.
func (s *Service) GetSpec(ctx context.Context, channelID string) (*ChannelSpec, bool, error) {
	if s == nil || s.Registry == nil {
		return nil, false, fmt.Errorf("channel registry is required")
	}
	if channelID == "" {
		return nil, false, fmt.Errorf("channel id is required")
	}
	spec, ok := s.Registry.GetSpec(ctx, channelID)
	return spec, ok, nil
}

// List returns info for all registered channels.
func (s *Service) List(ctx context.Context) ([]Info, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("channel registry is required")
	}
	return s.Registry.List(ctx), nil
}

// GetRuntimeChannel returns a connected runtime channel by id.
func (s *Service) GetRuntimeChannel(ctx context.Context, channelID string) (RuntimeChannel, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("channel registry is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	return s.Registry.GetRuntimeChannel(ctx, channelID)
}
