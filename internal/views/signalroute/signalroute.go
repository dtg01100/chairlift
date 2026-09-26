// Package signalroute lets many widgets share one GTK signal callback.
//
// puregotk turns every Connect* callback into a purego trampoline drawn from
// a fixed table of 2000 slots and panics when the table is full. A slot is
// reused only when the same func variable address is connected again; it is
// never released when the widget is destroyed. A view that connects a fresh
// closure per row inside a refresh therefore leaks slots on every reload
// until the application crashes.
//
// A Table instead maps each emitting object's address to its action. The
// view connects one long-lived func variable to every widget in a list, that
// callback calls Dispatch with the emitter's address, and a reload calls Clear
// before rebuilding. The number of trampolines is then one per list, not one
// per row per reload.
//
// The package imports nothing outside the standard library so it can be
// tested on a headless host; it is generic over the argument the signal
// carries, so no widget type appears here. A Table is not safe for
// concurrent use: every call happens on the GTK main thread, where signals
// are emitted and lists are rebuilt.
package signalroute

// Table maps emitter addresses to actions. The zero value is ready to use.
type Table[A any] struct {
	actions map[uintptr]func(A)
}

// Bind records action for the object at key, replacing any earlier binding:
// a destroyed widget's address may be reused by the next widget built.
func (t *Table[A]) Bind(key uintptr, action func(A)) {
	if t.actions == nil {
		t.actions = make(map[uintptr]func(A))
	}
	t.actions[key] = action
}

// Unbind forgets the action for key, when its widget is removed on its own.
func (t *Table[A]) Unbind(key uintptr) {
	delete(t.actions, key)
}

// Clear forgets every binding, when the whole list is rebuilt.
func (t *Table[A]) Clear() {
	clear(t.actions)
}

// Len reports how many emitters are bound.
func (t *Table[A]) Len() int {
	return len(t.actions)
}

// Dispatch runs the action bound to key with arg and reports whether one was
// bound. An emitter from a cleared or unbound list runs nothing.
func (t *Table[A]) Dispatch(key uintptr, arg A) bool {
	action, ok := t.actions[key]
	if ok {
		action(arg)
	}
	return ok
}

// DispatchOnce is Dispatch for an emitter that fires once, such as a dialog's
// response: the binding is removed before the action runs.
func (t *Table[A]) DispatchOnce(key uintptr, arg A) bool {
	action, ok := t.actions[key]
	if ok {
		delete(t.actions, key)
		action(arg)
	}
	return ok
}
