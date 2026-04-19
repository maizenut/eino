package react

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type MockToolCallingModel struct {
	Tools []*schema.ToolInfo
}

func NewMockToolCallingModel() *MockToolCallingModel {
	return &MockToolCallingModel{}
}

func (m *MockToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	for i := len(input) - 1; i >= 0; i-- {
		msg := input[i]
		if msg == nil {
			continue
		}
		if msg.Role == schema.Tool {
			return schema.AssistantMessage(fmt.Sprintf("tool result received: %s", msg.Content), nil), nil
		}
		if msg.Role == schema.User {
			toolName := "lookup"
			if len(m.Tools) > 0 && m.Tools[0] != nil && m.Tools[0].Name != "" {
				toolName = m.Tools[0].Name
			}
			args, _ := json.Marshal(map[string]any{"query": msg.Content})
			return schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "react_call_1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      toolName,
					Arguments: string(args),
				},
			}}), nil
		}
	}
	return schema.AssistantMessage("", nil), nil
}

func (m *MockToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *MockToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cloned := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			cloned = append(cloned, nil)
			continue
		}
		copied := *tool
		cloned = append(cloned, &copied)
	}
	return &MockToolCallingModel{Tools: cloned}, nil
}
