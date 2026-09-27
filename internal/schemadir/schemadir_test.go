package schemadir

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// caskInstall lays out a cask-style install: a binary with data/*.gschema.xml
// beside it. It returns the binary's path.
func caskInstall(t *testing.T, schemas map[string]string) string {
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
	binary := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
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

// An upgrade whose schemas changed recompiles into the same directory, so a
// Livery rotation unit installed before the upgrade still finds it, and drops
// a schema the new release no longer ships.
func TestPrepareRecompilesInPlaceWhenSchemasChange(t *testing.T) {
	cache := t.TempDir()
	oldBinary := caskInstall(t, map[string]string{
		"a.gschema.xml":       "<schemalist>old</schemalist>",
		"dropped.gschema.xml": "<schemalist/>",
	})
	calls := stub(t, oldBinary, cache)
	old, err := Prepare()
	if err != nil || old == "" {
		t.Fatalf("Prepare() = %q, %v", old, err)
	}

	newBinary := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist>new</schemalist>"})
	executable = func() (string, error) { return newBinary, nil }
	current, err := Prepare()
	if err != nil || current != old {
		t.Fatalf("Prepare() after upgrade = %q, %v; want the same directory %q", current, err, old)
	}
	if *calls != 2 {
		t.Errorf("compiler ran %d times, want 2 (once per distinct source set)", *calls)
	}
	source, err := os.ReadFile(filepath.Join(current, "a.gschema.xml"))
	if err != nil || string(source) != "<schemalist>new</schemalist>" {
		t.Errorf("a.gschema.xml = %q, %v; want the upgraded source", source, err)
	}
	if _, err := os.Stat(filepath.Join(current, "dropped.gschema.xml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dropped schema still present (err=%v)", err)
	}
}

// A source build has no data/ beside the binary; there is nothing to prepare.
func TestPrepareWithoutShippedSchemas(t *testing.T) {
	binary := caskInstall(t, nil)
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
	binary := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
	stub(t, binary, t.TempDir())
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }

	dir, err := Prepare()
	if err != nil || dir != "" {
		t.Fatalf("Prepare() = %q, %v; want nothing to do without a compiler", dir, err)
	}
}

// A failed compile returns an error and leaves a previous good install
// exactly as it was, so the launch keeps working schemas.
func TestPrepareFailedCompileKeepsThePreviousInstall(t *testing.T) {
	cache := t.TempDir()
	good := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist>good</schemalist>"})
	stub(t, good, cache)
	dir, err := Prepare()
	if err != nil || dir == "" {
		t.Fatalf("Prepare() = %q, %v", dir, err)
	}

	broken := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist>broken</schemalist>"})
	executable = func() (string, error) { return broken, nil }
	compile = func(context.Context, string, string) error { return errors.New("invalid schema") }
	if got, err := Prepare(); err == nil || got != "" {
		t.Fatalf("Prepare() = %q, %v; want an error", got, err)
	}

	source, err := os.ReadFile(filepath.Join(dir, "a.gschema.xml"))
	if err != nil || string(source) != "<schemalist>good</schemalist>" {
		t.Errorf("a.gschema.xml = %q, %v; want the previous good source", source, err)
	}
	executable = func() (string, error) { return good, nil }
	if got, err := Prepare(); err != nil || got != dir {
		t.Errorf("Prepare() for the previous version = %q, %v; want its install reused", got, err)
	}
}

// A first launch whose compile fails leaves no stamp a later launch would
// mistake for a good install.
func TestPrepareFailedFirstCompileLeavesNoStamp(t *testing.T) {
	cache := t.TempDir()
	binary := caskInstall(t, map[string]string{"a.gschema.xml": "<schemalist/>"})
	stub(t, binary, cache)
	compile = func(context.Context, string, string) error { return errors.New("invalid schema") }

	if dir, err := Prepare(); err == nil || dir != "" {
		t.Fatalf("Prepare() = %q, %v; want an error", dir, err)
	}
	target, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, stampName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stamp present after a failed compile (err=%v)", err)
	}
	staged, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".schemas-staging-*"))
	if len(staged) != 0 {
		t.Errorf("staging directories left behind: %v", staged)
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
	binary := caskInstall(t, schemas)
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
