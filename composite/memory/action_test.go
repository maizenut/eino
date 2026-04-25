package memory

import "testing"

func TestActionKindClassification(t *testing.T) {
	readActions := []ActionKind{ActionReckon, ActionPonder, ActionInspire}
	for _, kind := range readActions {
		if !IsReadAction(kind) {
			t.Fatalf("IsReadAction(%q) = false, want true", kind)
		}
		if IsWriteAction(kind) {
			t.Fatalf("IsWriteAction(%q) = true, want false", kind)
		}
	}

	writeActions := []ActionKind{ActionSummary, ActionImprint}
	for _, kind := range writeActions {
		if !IsWriteAction(kind) {
			t.Fatalf("IsWriteAction(%q) = false, want true", kind)
		}
		if IsReadAction(kind) {
			t.Fatalf("IsReadAction(%q) = true, want false", kind)
		}
	}

	if IsReadAction(ActionForget) || IsWriteAction(ActionForget) {
		t.Fatal("ActionForget should be neither read nor write")
	}
}
