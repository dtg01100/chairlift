// Package installcheck contains headless, gate-enforced regression tests for
// repository-level documentation, workflow, installation, packaging, and
// PolicyKit contracts.
//
// This package holds no logic reachable from cmd/ or any other internal/
// package, only shared test-time helpers. It imports no GTK bindings, directly
// or transitively, so a _test.go living here never trips the
// gtk-headless-tests.md constraint (docs/agents/skills/gtk-headless-tests.md),
// and it lives under internal/... so it is actually exercised by gates_chunk,
// make ci, and CI's `go test ./internal/...` filter, per
// docs/agents/skills/gate-test-scope-is-internal-only.md.
package installcheck

import (
	"path/filepath"
	"runtime"
)

// RepoRoot returns the absolute path to the repository root. It is computed
// from this source file's own location (via runtime.Caller) rather than the
// test binary's working directory: `go test` runs each package's tests with
// the package directory as cwd, not the repo root, and internal/installcheck
// sits two directories below the repo root
// (<root>/internal/installcheck/installcheck.go).
func RepoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// GoreleaserConfig is a minimal, forward-compatible subset of the fields in
// .goreleaser.yaml this package's tests care about. yaml.v3's default
// unmarshaling silently ignores every field not named here, so this struct
// does not need to (and deliberately does not) mirror the whole schema.
//
// There is deliberately no top-level metadata: block here. GoReleaser OSS
// (unlike Pro) does not support metadata.description/homepage/license/
// maintainers — see the "switch to plain GitHub Releases" note in
// docs/design/package-managers.md — so the repository URL lives only in
// release.footer's literal text, and the license travels as the LICENSE file
// every release archive ships.
type GoreleaserConfig struct {
	Builds   []BuildConfig   `yaml:"builds"`
	Archives []ArchiveConfig `yaml:"archives"`
	Release  ReleaseConfig   `yaml:"release"`
}

// BuildConfig is the subset of a builds[] entry that decides which executable
// a release archive carries and under what file name.
type BuildConfig struct {
	ID     string `yaml:"id"`
	Binary string `yaml:"binary"`
}

// ArchiveConfig is the subset of an archives[] entry relevant to what the
// release tarball — the artifact the Homebrew cask installs — contains. An
// empty IDs list means GoReleaser packs every build's binary.
type ArchiveConfig struct {
	IDs   []string `yaml:"ids"`
	Files []string `yaml:"files"`
}

// ReleaseConfig is the subset of the top-level release: block relevant to the
// generated release notes. Footer holds the raw, unrendered Go template text
// goreleaser expands at release time; the tests assert on that template text
// (they never render it, and never invoke goreleaser, which is not installed
// on the gate host or in make ci).
type ReleaseConfig struct {
	Footer string `yaml:"footer"`
}
