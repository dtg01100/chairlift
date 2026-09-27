package schemadir

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// install lays out a cask-style install: a binary with data/*.gschema.xml
// beside it. It returns the binary's path.
func install(t *testing.T, schemas map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range schemas {
		if err := os.WriteFile(filepath.Join(root, "data", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "chairlift")
	if err := os.WriteFile(binary, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return binary
}

// stub points Prepare at binary and cache, and replaces the compiler with
// one that records calls and writes gschemas.compiled.
func stub(t *testing.T, binary, cache string) *int {
	t.Helper()
	calls := 0
	previous := struct {
		executable func() (string, error)
		cacheDir   func() (string, error)
		lookPath   func(string) (string, error)
		compile    func(context.Context, string, string) error
	}{executable, cacheDir, lookPath, compile}
	executable = func() (string, error) { return binary, nil }
	cacheDir = func() (string, error) { return cache, nil }
	lookPath = func(string) (string, error) { return "/usr/bin/glib-compile-schemas", nil }
	compile = func(_ context.Context, _, dir string) error {
		calls++
		return os.WriteFile(filepath.Join(dir, "gschemas.compiled"), []byte("compiled"), 0o644)
	}
	t.Cleanup(func() {
		executable, cacheDir, lookPath, compile = previous.executable, previous.cacheDir, previous.lookPath, previous.compile
	})
	return &calls
}

// A cask install compiles once; every later launch of the same version reuses
// the directory without running the compiler again.
func TestPrepareCompilesOnceAndReuses(t *testing.T) {
	binary := install(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
	calls := stub(t, binary, t.TempDir())

	first, err := Prepare()
	if err != nil || first == "" {
		t.Fatalf("Prepare() = %q, %v; want a compiled directory", first, err)
	}
	if _, err := os.Stat(filepath.Join(first, "a.gschema.xml")); err != nil {
		t.Errorf("compiled directory lacks the schema source: %v", err)
	}
	second, err := Prepare()
	if err != nil || second != first {
		t.Fatalf("second Prepare() = %q, %v; want %q", second, err, first)
	}
	if *calls != 1 {
		t.Errorf("compiler ran %d times, want 1", *calls)
	}
}

// An upgrade whose schema changed compiles a new directory and removes the
// one compiled for the old version.
func TestPrepareRecompilesWhenSchemasChange(t *testing.T) {
	cache := t.TempDir()
	oldBinary := install(t, map[string]string{"a.gschema.xml": "<schemalist>old</schemalist>"})
	stub(t, oldBinary, cache)
	old, err := Prepare()
	if err != nil || old == "" {
		t.Fatalf("Prepare() = %q, %v", old, err)
	}

	newBinary := install(t, map[string]string{"a.gschema.xml": "<schemalist>new</schemalist>"})
	executable = func() (string, error) { return newBinary, nil }
	current, err := Prepare()
	if err != nil || current == "" || current == old {
		t.Fatalf("Prepare() after upgrade = %q, %v; want a new directory other than %q", current, err, old)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale directory %q still present (err=%v)", old, err)
	}
}

// A source build has no data/ beside the binary; there is nothing to prepare.
func TestPrepareWithoutShippedSchemas(t *testing.T) {
	binary := install(t, nil)
	calls := stub(t, binary, t.TempDir())

	dir, err := Prepare()
	if err != nil || dir != "" {
		t.Fatalf("Prepare() = %q, %v; want nothing to do", dir, err)
	}
	if *calls != 0 {
		t.Errorf("compiler ran %d times, want 0", *calls)
	}
}

// A host without glib-compile-schemas degrades to today's behavior: the
// schema is reported missing rather than the launch failing.
func TestPrepareWithoutCompiler(t *testing.T) {
	binary := install(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
	stub(t, binary, t.TempDir())
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }

	dir, err := Prepare()
	if err != nil || dir != "" {
		t.Fatalf("Prepare() = %q, %v; want nothing to do without a compiler", dir, err)
	}
}

// A failed compile returns an error and leaves no directory a later launch
// would mistake for a good one.
func TestPrepareFailedCompileLeavesNoDirectory(t *testing.T) {
	cache := t.TempDir()
	binary := install(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
	stub(t, binary, cache)
	compile = func(context.Context, string, string) error { return errors.New("invalid schema") }

	dir, err := Prepare()
	if err == nil || dir != "" {
		t.Fatalf("Prepare() = %q, %v; want an error", dir, err)
	}
	entries, _ := os.ReadDir(filepath.Join(cache, "chairlift", "schemas"))
	if len(entries) != 0 {
		t.Errorf("cache holds %d entries after a failed compile, want none", len(entries))
	}
}

// The schemas ChairLift ships compile with the real tool into a directory
// GLib can load. Skipped where glib-compile-schemas is not installed.
func TestPrepareCompilesTheShippedSchemas(t *testing.T) {
	compiler, err := exec.LookPath("glib-compile-schemas")
	if err != nil {
		t.Skip("glib-compile-schemas not installed")
	}
	shipped, err := filepath.Glob(filepath.Join("..", "..", "data", "*.gschema.xml"))
	if err != nil || len(shipped) != 3 {
		t.Fatalf("shipped schemas = %v, %v; want the three ChairLift schemas", shipped, err)
	}
	schemas := make(map[string]string)
	for _, path := range shipped {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		schemas[filepath.Base(path)] = string(data)
	}
	binary := install(t, schemas)
	stub(t, binary, t.TempDir())
	lookPath = func(string) (string, error) { return compiler, nil }
	compile = runCompile

	dir, err := Prepare()
	if err != nil || dir == "" {
		t.Fatalf("Prepare() = %q, %v; want a compiled directory", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gschemas.compiled")); err != nil {
		t.Errorf("no gschemas.compiled: %v", err)
	}
}
