// Package schemadir makes ChairLift's GSettings schemas available when they
// are not installed system-wide.
//
// ChairLift is distributed through a Homebrew cask, and a cask cannot write
// /usr/share/glib-2.0/schemas. The cask does unpack the release archive,
// whose data/ directory carries the schema sources beside the binary. Prepare
// compiles those sources once into a per-user cache directory, keyed by their
// content, and returns it for GSETTINGS_SCHEMA_DIR. GLib searches that
// directory before the system ones, so the schemas always match the running
// binary, whatever version the operating system image ships.
//
// The package imports no puregotk, so its tests run headlessly (ADR-0007).
package schemadir

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EnvVar is the environment variable GLib reads schema directories from.
const EnvVar = "GSETTINGS_SCHEMA_DIR"

// compileTimeout bounds the one-time glib-compile-schemas run at startup.
const compileTimeout = 10 * time.Second

// Injection seams; production never reassigns them.
var (
	executable = os.Executable
	cacheDir   = os.UserCacheDir
	lookPath   = exec.LookPath
	compile    = runCompile
)

func runCompile(ctx context.Context, compiler, dir string) error {
	out, err := exec.CommandContext(ctx, compiler, dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", compiler, dir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Prepare returns a directory holding the compiled schemas shipped beside the
// running binary, compiling them on first use. It returns "" with a nil error
// when there is nothing to do: no schema sources beside the binary (a source
// build, or a system install whose schemas are already registered), or no
// glib-compile-schemas on this host.
func Prepare() (string, error) {
	exe, err := executable()
	if err != nil {
		return "", fmt.Errorf("schemadir: locating executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	sources, err := filepath.Glob(filepath.Join(filepath.Dir(exe), "data", "*.gschema.xml"))
	if err != nil || len(sources) == 0 {
		return "", nil
	}
	sort.Strings(sources)

	key, err := contentKey(sources)
	if err != nil {
		return "", err
	}

	base, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("schemadir: locating cache directory: %w", err)
	}
	root := filepath.Join(base, "chairlift", "schemas")
	target := filepath.Join(root, key)
	if _, err := os.Stat(filepath.Join(target, "gschemas.compiled")); err == nil {
		return target, nil
	}

	compiler, err := lookPath("glib-compile-schemas")
	if err != nil {
		return "", nil
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	staging, err := os.MkdirTemp(root, ".staging-")
	if err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("schemadir: %w", err)
		}
		if err := os.WriteFile(filepath.Join(staging, filepath.Base(source)), data, 0o644); err != nil {
			return "", fmt.Errorf("schemadir: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()
	if err := compile(ctx, compiler, staging); err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(staging, "gschemas.compiled")); err != nil {
		return "", fmt.Errorf("schemadir: compiler produced no gschemas.compiled: %w", err)
	}

	// A concurrent launch may have finished the same key first; its result
	// is identical, so either one is fine.
	if err := os.Rename(staging, target); err != nil {
		if _, statErr := os.Stat(filepath.Join(target, "gschemas.compiled")); statErr != nil {
			return "", fmt.Errorf("schemadir: %w", err)
		}
	}
	removeStale(root, key)
	return target, nil
}

// contentKey identifies one set of schema sources by name and content, so a
// cask upgrade that changes a schema compiles a fresh directory and a
// reinstall of the same version reuses the old one.
func contentKey(sources []string) (string, error) {
	hash := sha256.New()
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("schemadir: %w", err)
		}
		// hash.Hash's Write never returns an error.
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00", filepath.Base(source), len(data))
		_, _ = hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil))[:16], nil
}

// removeStale deletes directories compiled for earlier schema sources.
func removeStale(root, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != keep && !strings.HasPrefix(entry.Name(), ".staging-") {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}
