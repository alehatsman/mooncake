package windows_scheduled_task

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func makeTaskResult(info *WindowsScheduledTaskReverseInfo) *executor.Result {
	r := executor.NewResult()
	if info != nil {
		r.ReverseData = info
	}
	return r
}

func baseTaskStep(name string) *config.Step {
	return &config.Step{
		Name: name,
		WindowsScheduledTask: &config.WindowsScheduledTask{
			Name:  name,
			State: statePresent,
		},
	}
}

func TestTaskReverse_NilStep(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, nil, makeTaskResult(nil))
	if err == nil {
		t.Fatal("expected error for nil step")
	}
}

func TestTaskReverse_NilResult(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, baseTaskStep("T"), nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestTaskReverse_WrongReverseDataType(t *testing.T) {
	r := executor.NewResult()
	r.ReverseData = "not-the-right-type"
	_, err := (&Handler{}).Reverse(nil, baseTaskStep("T"), r)
	if err == nil {
		t.Fatal("expected error for wrong ReverseData type")
	}
}

func TestTaskReverse_NilReverseData_Noop(t *testing.T) {
	step, err := (&Handler{}).Reverse(nil, baseTaskStep("T"), makeTaskResult(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step != nil {
		t.Fatalf("expected nil step for noop; got %+v", step)
	}
}

func TestTaskReverse_PresentCreate_ReturnsAbsent(t *testing.T) {
	info := &WindowsScheduledTaskReverseInfo{
		AppliedState: statePresent,
		PriorExisted: false,
		TaskName:     "MyTask",
	}
	step, err := (&Handler{}).Reverse(nil, baseTaskStep("MyTask"), makeTaskResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step == nil || step.WindowsScheduledTask == nil {
		t.Fatal("expected a WindowsScheduledTask step")
	}
	if step.WindowsScheduledTask.State != stateAbsent {
		t.Errorf("state = %q; want absent", step.WindowsScheduledTask.State)
	}
	if step.WindowsScheduledTask.Name != "MyTask" {
		t.Errorf("name = %q; want MyTask", step.WindowsScheduledTask.Name)
	}
}

func TestTaskReverse_PresentUpdate_Unsupported(t *testing.T) {
	info := &WindowsScheduledTaskReverseInfo{
		AppliedState: statePresent,
		PriorExisted: true,
		TaskName:     "MyTask",
	}
	_, err := (&Handler{}).Reverse(nil, baseTaskStep("MyTask"), makeTaskResult(info))
	if err == nil {
		t.Fatal("expected error for update-rollback (not yet supported)")
	}
	if !strings.Contains(err.Error(), "not yet supported") {
		t.Errorf("error should mention 'not yet supported'; got %v", err)
	}
}

func TestTaskReverse_AbsentDelete_Unsupported(t *testing.T) {
	info := &WindowsScheduledTaskReverseInfo{
		AppliedState: stateAbsent,
		PriorExisted: true,
		TaskName:     "MyTask",
	}
	_, err := (&Handler{}).Reverse(nil, baseTaskStep("MyTask"), makeTaskResult(info))
	if err == nil {
		t.Fatal("expected error for delete-rollback (not yet supported)")
	}
	if !strings.Contains(err.Error(), "not yet supported") {
		t.Errorf("error should mention 'not yet supported'; got %v", err)
	}
}

func TestTaskReverse_UnknownState_Error(t *testing.T) {
	info := &WindowsScheduledTaskReverseInfo{
		AppliedState: "bogus",
		TaskName:     "X",
	}
	_, err := (&Handler{}).Reverse(nil, baseTaskStep("X"), makeTaskResult(info))
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}
