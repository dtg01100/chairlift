package signalroute

import "testing"

func TestDispatchRunsOnlyTheActionBoundToTheEmitter(t *testing.T) {
	var table Table[string]
	var got []string
	table.Bind(1, func(arg string) { got = append(got, "one:"+arg) })
	table.Bind(2, func(arg string) { got = append(got, "two:"+arg) })

	if !table.Dispatch(2, "clicked") {
		t.Fatal("Dispatch(2) = false, want true for a bound emitter")
	}
	if len(got) != 1 || got[0] != "two:clicked" {
		t.Fatalf("actions run = %v, want [two:clicked]", got)
	}
}

func TestDispatchAfterClearRunsNothing(t *testing.T) {
	var table Table[struct{}]
	ran := false
	table.Bind(7, func(struct{}) { ran = true })
	table.Clear()

	if table.Dispatch(7, struct{}{}) {
		t.Fatal("Dispatch after Clear = true, want false")
	}
	if ran {
		t.Fatal("an action from a cleared list ran")
	}
	if table.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", table.Len())
	}
}

// A destroyed widget's address can be handed to the next widget built in the
// same list; the new widget's action must win.
func TestBindReplacesAReusedEmitterKey(t *testing.T) {
	var table Table[struct{}]
	var got string
	table.Bind(9, func(struct{}) { got = "old" })
	table.Bind(9, func(struct{}) { got = "new" })

	table.Dispatch(9, struct{}{})
	if got != "new" {
		t.Fatalf("action run = %q, want the most recent binding", got)
	}
	if table.Len() != 1 {
		t.Fatalf("Len = %d, want 1", table.Len())
	}
}

func TestUnbindForgetsOneEmitter(t *testing.T) {
	var table Table[struct{}]
	kept := false
	table.Bind(1, func(struct{}) { t.Fatal("unbound action ran") })
	table.Bind(2, func(struct{}) { kept = true })
	table.Unbind(1)

	if table.Dispatch(1, struct{}{}) {
		t.Fatal("Dispatch of an unbound emitter = true, want false")
	}
	table.Dispatch(2, struct{}{})
	if !kept {
		t.Fatal("unbinding one emitter dropped another")
	}
}

// A dialog answers once; its action must not survive to a second emission
// from whatever object is later allocated at the same address.
func TestDispatchOnceForgetsTheEmitter(t *testing.T) {
	var table Table[string]
	calls := 0
	table.Bind(3, func(string) { calls++ })

	if !table.DispatchOnce(3, "confirm") {
		t.Fatal("DispatchOnce of a bound emitter = false, want true")
	}
	if table.DispatchOnce(3, "confirm") {
		t.Fatal("second DispatchOnce = true, want false")
	}
	if calls != 1 {
		t.Fatalf("action ran %d times, want 1", calls)
	}
}
