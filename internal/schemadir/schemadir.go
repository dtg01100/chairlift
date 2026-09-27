// Package schemadir makes ChairLift's GSettings schemas available when they
// are not installed system-wide.
//
// ChairLift is distributed through a Homebrew cask, and a cask cannot write
// /usr/share/glib-2.0/schemas. The cask does unpack the release archive,
// whose data/ directory carries the schema sources beside the binary. Prepare
// compiles those sources into one fixed per-user directory and returns it for
// GSETTINGS_SCHEMA_DIR. GLib searches that directory before the system ones,
// so the schemas always match the running binary, whatever version the
// operating system image ships.
//
// The directory never moves. The Livery rotation unit records the
// GSETTINGS_SCHEMA_DIR it was installed with, so a path that changed with
// each release would leave that unit pointing at a directory a later upgrade
// had removed. A stamp file inside the directory records which sources it was
// compiled from; a changed stamp recompiles in place.
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

// compiledName is the cache file glib-compile-schemas writes.
const compiledName = "gschemas.compiled"

// stampName records the content key the directory was compiled from.
const stampName = ".sources-stamp"

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

// Dir returns the fixed directory Prepare compiles into:
// $XDG_CACHE_HOME/chairlift/schemas.
func Dir() (string, error) {
	base, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("schemadir: locating cache directory: %w", err)
	}
	return filepath.Join(base, "chairlift", "schemas"), nil
}

// Prepare returns the directory holding the compiled schemas shipped beside
// the running binary, compiling them when they changed. It returns "" with a
// nil error when there is nothing to do: no schema sources beside the binary
// (a source build, or a system install whose schemas are already registered),
// or no glib-compile-schemas on this host.
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

	target, err := Dir()
	if err != nil {
		return "", err
	}
	if current(target, key) {
		return target, nil
	}

	compiler, err := lookPath("glib-compile-schemas")
	if err != nil {
		return "", nil
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".schemas-staging-")
	if err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	for _, source := range sources {
		if err := copyFile(source, filepath.Join(staging, filepath.Base(source))); err != nil {
			return "", err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()
	if err := compile(ctx, compiler, staging); err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(staging, compiledName)); err != nil {
		return "", fmt.Errorf("schemadir: compiler produced no %s: %w", compiledName, err)
	}
	if err := os.WriteFile(filepath.Join(staging, stampName), []byte(key+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("schemadir: %w", err)
	}

	if err := install(staging, target, sources); err != nil {
		return "", err
	}
	return target, nil
}

// current reports whether target was compiled from the sources key names.
func current(target, key string) bool {
	stamp, err := os.ReadFile(filepath.Join(target, stampName))
	if err != nil || strings.TrimSpace(string(stamp)) != key {
		return false
	}
	_, err = os.Stat(filepath.Join(target, compiledName))
	return err == nil
}

// install moves the staged sources, then the compiled cache, then the stamp
// into target. Each is an atomic rename within one filesystem, and the stamp
// goes last, so an interrupted install is recompiled on the next launch
// rather than trusted.
func install(staging, target string, sources []string) error {
	keep := map[string]bool{compiledName: true, stampName: true}
	for _, source := range sources {
		name := filepath.Base(source)
		keep[name] = true
		if err := os.Rename(filepath.Join(staging, name), filepath.Join(target, name)); err != nil {
			return fmt.Errorf("schemadir: %w", err)
		}
	}
	if err := os.Rename(filepath.Join(staging, compiledName), filepath.Join(target, compiledName)); err != nil {
		return fmt.Errorf("schemadir: %w", err)
	}
	if err := os.Rename(filepath.Join(staging, stampName), filepath.Join(target, stampName)); err != nil {
		return fmt.Errorf("schemadir: %w", err)
	}

	// A schema a later release dropped must not linger beside the new cache.
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !keep[entry.Name()] && strings.HasSuffix(entry.Name(), ".gschema.xml") {
			_ = os.Remove(filepath.Join(target, entry.Name()))
		}
	}
	return nil
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("schemadir: %w", err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return fmt.Errorf("schemadir: %w", err)
	}
	return nil
}

// contentKey identifies one set of schema sources by name and content, so a
// cask upgrade that changes a schema recompiles and a reinstall of the same
// version reuses the compiled cache.
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
	return hex.EncodeToString(hash.Sum(nil)), nil
}
