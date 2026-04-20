package skill

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func TestRunnableCanBindToReactAgent(t *testing.T) {
	ctx := context.Background()

	skillTool := &recordingSkillTool{
		info: &schema.ToolInfo{
			Name: "skill_lookup",
			Desc: "Look up skill knowledge.",
		},
		result: `{"status":"ok"}`,
	}
	runnable := &resolvedSkill{
		info:        Info{Name: "lookup-skill"},
		instruction: "You are a skill-enabled agent. Always use the bound skill tools when needed.",
		tools:       []tool.BaseTool{skillTool},
	}

	fakeModel := &recordingToolCallingModel{
		firstResponse: schema.AssistantMessage("", []schema.ToolCall{
			{
				ID:   "call_skill_lookup",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "skill_lookup",
					Arguments: `{"query":"where is the answer"}`,
				},
			},
		}),
		secondResponse: schema.AssistantMessage("final answer from skill", nil),
	}

	agentUnderTest := &skillReactLoop{
		model:       fakeModel,
		instruction: runnable.instruction,
	}

	got, err := agentUnderTest.generate(ctx, runnable.tools, []*schema.Message{
		schema.UserMessage("Please use the skill to answer."),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if got.Content != "final answer from skill" {
		t.Fatalf("final content = %q, want final answer from skill", got.Content)
	}

	if fakeModel.generateCalls != 2 {
		t.Fatalf("generate calls = %d, want 2", fakeModel.generateCalls)
	}

	if len(fakeModel.firstCallTools) != 1 {
		t.Fatalf("first call tool count = %d, want 1", len(fakeModel.firstCallTools))
	}
	if fakeModel.firstCallTools[0].Name != "skill_lookup" {
		t.Fatalf("first call tool name = %q, want skill_lookup", fakeModel.firstCallTools[0].Name)
	}

	if len(fakeModel.firstCallMessages) < 2 {
		t.Fatalf("first call message count = %d, want >= 2", len(fakeModel.firstCallMessages))
	}
	if fakeModel.firstCallMessages[0].Role != schema.System {
		t.Fatalf("first call first role = %s, want system", fakeModel.firstCallMessages[0].Role)
	}
	if fakeModel.firstCallMessages[0].Content != runnable.instruction {
		t.Fatalf("system instruction = %q, want %q", fakeModel.firstCallMessages[0].Content, runnable.instruction)
	}
	if fakeModel.firstCallMessages[1].Role != schema.User {
		t.Fatalf("first call second role = %s, want user", fakeModel.firstCallMessages[1].Role)
	}

	if len(fakeModel.secondCallMessages) < 4 {
		t.Fatalf("second call message count = %d, want >= 4", len(fakeModel.secondCallMessages))
	}
	toolMsg := fakeModel.secondCallMessages[len(fakeModel.secondCallMessages)-1]
	if toolMsg.Role != schema.Tool {
		t.Fatalf("last message role = %s, want tool", toolMsg.Role)
	}
	if toolMsg.ToolName != "skill_lookup" {
		t.Fatalf("tool message name = %q, want skill_lookup", toolMsg.ToolName)
	}
	if toolMsg.Content != `{"status":"ok"}` {
		t.Fatalf("tool message content = %q", toolMsg.Content)
	}

	if skillTool.invocations != 1 {
		t.Fatalf("tool invocations = %d, want 1", skillTool.invocations)
	}
	if skillTool.lastArguments != `{"query":"where is the answer"}` {
		t.Fatalf("tool arguments = %q", skillTool.lastArguments)
	}
}

// skillReactLoop is a minimal ReAct-style loop for testing skill binding
// without depending on eino/flow/agent/react.
type skillReactLoop struct {
	model       model.ToolCallingChatModel
	instruction string
}

func (l *skillReactLoop) generate(ctx context.Context, tools []tool.BaseTool, input []*schema.Message) (*schema.Message, error) {
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	toolMap := make(map[string]tool.InvokableTool, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("get tool info: %w", err)
		}
		toolInfos = append(toolInfos, info)
		if inv, ok := t.(tool.InvokableTool); ok {
			toolMap[info.Name] = inv
		}
	}

	boundModel, err := l.model.WithTools(toolInfos)
	if err != nil {
		return nil, fmt.Errorf("bind tools to model: %w", err)
	}

	messages := make([]*schema.Message, 0, len(input)+2)
	if l.instruction != "" {
		messages = append(messages, schema.SystemMessage(l.instruction))
	}
	messages = append(messages, input...)

	for i := 0; i < 10; i++ {
		msg, err := boundModel.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("model generate step %d: %w", i, err)
		}
		if len(msg.ToolCalls) == 0 {
			return msg, nil
		}
		messages = append(messages, msg)
		for _, tc := range msg.ToolCalls {
			inv, ok := toolMap[tc.Function.Name]
			if !ok {
				return nil, fmt.Errorf("unknown tool %q", tc.Function.Name)
			}
			result, err := inv.InvokableRun(ctx, tc.Function.Arguments)
			if err != nil {
				return nil, fmt.Errorf("tool %q: %w", tc.Function.Name, err)
			}
			messages = append(messages, schema.ToolMessage(result, tc.ID, schema.WithToolName(tc.Function.Name)))
		}
	}
	return nil, fmt.Errorf("react loop exceeded max steps")
}

type recordingSkillTool struct {
	info          *schema.ToolInfo
	result        string
	invocations   int
	lastArguments string
}

func (t *recordingSkillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	_ = ctx
	return t.info, nil
}

func (t *recordingSkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	_ = ctx
	_ = opts
	t.invocations++
	t.lastArguments = argumentsInJSON
	return t.result, nil
}

type recordingToolCallingModel struct {
	firstResponse  *schema.Message
	secondResponse *schema.Message

	generateCalls      int
	firstCallMessages  []*schema.Message
	secondCallMessages []*schema.Message
	firstCallTools     []*schema.ToolInfo
	secondCallTools    []*schema.ToolInfo
}

func (m *recordingToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	m.generateCalls++

	copiedMessages := cloneMessages(input)

	switch m.generateCalls {
	case 1:
		m.firstCallMessages = copiedMessages
		return m.firstResponse, nil
	case 2:
		m.secondCallMessages = copiedMessages
		return m.secondResponse, nil
	default:
		return nil, errors.New("unexpected extra model call")
	}
}

func (m *recordingToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *recordingToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.firstCallTools = cloneToolInfos(tools)
	return m, nil
}

func cloneMessages(in []*schema.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(in))
	for _, msg := range in {
		if msg == nil {
			out = append(out, nil)
			continue
		}
		copied := *msg
		if len(msg.ToolCalls) > 0 {
			copied.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
		}
		out = append(out, &copied)
	}
	return out
}

func cloneToolInfos(in []*schema.ToolInfo) []*schema.ToolInfo {
	out := make([]*schema.ToolInfo, 0, len(in))
	for _, info := range in {
		if info == nil {
			out = append(out, nil)
			continue
		}
		copied := *info
		out = append(out, &copied)
	}
	return out
}

var _ tool.InvokableTool = (*recordingSkillTool)(nil)
var _ model.ToolCallingChatModel = (*recordingToolCallingModel)(nil)
