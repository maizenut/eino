package job

import declarative "github.com/cloudwego/eino/schema/declarative"

func taskIdentity(task *TaskSpec) string {
	if task == nil {
		return ""
	}
	if task.Info.ID != "" {
		return task.Info.ID
	}
	return task.Info.Name
}

func normalizeTriggerSpec(trigger *TriggerSpec) *TriggerSpec {
	if trigger == nil {
		return &TriggerSpec{Type: TriggerTypeManual}
	}
	cloned := cloneTriggerSpec(trigger)
	if cloned.Type == "" {
		cloned.Type = TriggerTypeManual
	}
	return cloned
}

func cloneJobSpec(spec *JobSpec) *JobSpec {
	if spec == nil {
		return nil
	}
	return &JobSpec{Task: cloneTaskSpec(spec.Task), Trigger: cloneTriggerSpec(spec.Trigger)}
}

func cloneTaskSpec(task *TaskSpec) *TaskSpec {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.Info = cloneTaskInfo(task.Info)
	cloned.Input = copyMap(task.Input)
	cloned.Metadata = copyMap(task.Metadata)
	cloned.TargetRef = cloneRef(task.TargetRef)
	return &cloned
}

func cloneTaskInfo(info TaskInfo) TaskInfo {
	info.Tags = append([]string(nil), info.Tags...)
	info.Metadata = copyMap(info.Metadata)
	return info
}

func cloneTriggerSpec(trigger *TriggerSpec) *TriggerSpec {
	if trigger == nil {
		return nil
	}
	cloned := *trigger
	if trigger.At != nil {
		at := *trigger.At
		cloned.At = &at
	}
	cloned.Payload = copyMap(trigger.Payload)
	cloned.Metadata = copyMap(trigger.Metadata)
	if trigger.Retry != nil {
		retry := *trigger.Retry
		cloned.Retry = &retry
	}
	if trigger.Concurrency != nil {
		concurrency := *trigger.Concurrency
		cloned.Concurrency = &concurrency
	}
	return &cloned
}

func cloneTriggerEvent(event *TriggerEvent) *TriggerEvent {
	if event == nil {
		return nil
	}
	cloned := *event
	if event.ScheduledAt != nil {
		scheduledAt := *event.ScheduledAt
		cloned.ScheduledAt = &scheduledAt
	}
	cloned.Payload = copyMap(event.Payload)
	return &cloned
}

func cloneRunResult(result *RunResult) *RunResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Event = cloneTriggerEvent(result.Event)
	cloned.Metadata = copyMap(result.Metadata)
	if result.StartedAt != nil {
		startedAt := *result.StartedAt
		cloned.StartedAt = &startedAt
	}
	if result.FinishedAt != nil {
		finishedAt := *result.FinishedAt
		cloned.FinishedAt = &finishedAt
	}
	return &cloned
}

func cloneRef(ref declarative.Ref) declarative.Ref {
	cloned := ref
	cloned.Args = copyMap(ref.Args)
	return cloned
}

func copyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeMaps(maps ...map[string]any) map[string]any {
	var total int
	for _, item := range maps {
		total += len(item)
	}
	if total == 0 {
		return nil
	}
	out := make(map[string]any, total)
	for _, item := range maps {
		for k, v := range item {
			out[k] = v
		}
	}
	return out
}
