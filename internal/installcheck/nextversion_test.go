package installcheck

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nextVersion runs scripts/next-version.sh inside a throwaway git repository
// holding exactly tags, with the calendar slot pinned to 26.09 so the answer
// does not depend on today's date.
func nextVersion(t *testing.T, tags []string, prerelease string) (string, error) {
	t.Helper()
	for _, tool := range []string{"git", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed; skipping next-version.sh check", tool)
		}
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "root")
	for _, tag := range tags {
		run("tag", tag)
	}

	args := []string{filepath.Join(RepoRoot(), "scripts", "next-version.sh")}
	if prerelease != "" {
		args = append(args, prerelease)
	}
	cmd := exec.Command("bash", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "NEXT_VERSION_SLOT=26.09")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// A prerelease belongs to the version it precedes: while vYY.MM.N has only
// prerelease tags, the next prerelease and the final release both stay on N.
// The script once bumped N past any existing tag, so `make bump PRE=alpha.3`
// after v26.09.0-alpha.2 tagged v26.09.1-alpha.3.
func TestNextVersionKeepsAnUnreleasedVersionNumber(t *testing.T) {
	cases := []struct {
		name       string
		tags       []string
		prerelease string
		want       string
	}{
		{"first release of the month", nil, "", "v26.09.0"},
		{"first prerelease of the month", nil, "alpha.1", "v26.09.0-alpha.1"},
		{"next prerelease of an unreleased version", []string{"v26.09.0-alpha.1", "v26.09.0-alpha.2"}, "alpha.3", "v26.09.0-alpha.3"},
		{"final release after its prereleases", []string{"v26.09.0-alpha.1", "v26.09.0-alpha.2"}, "", "v26.09.0"},
		{"prerelease after a final release", []string{"v26.09.0-alpha.1", "v26.09.0"}, "alpha.1", "v26.09.1-alpha.1"},
		{"release after a final release", []string{"v26.09.0"}, "", "v26.09.1"},
		{"other months do not count", []string{"v26.08.4", "v0.12.2"}, "", "v26.09.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextVersion(t, tc.tags, tc.prerelease)
			if err != nil {
				t.Fatalf("next-version.sh failed: %v\n%s", err, got)
			}
			if got != tc.want {
				t.Errorf("tags %v, prerelease %q: got %s, want %s", tc.tags, tc.prerelease, got, tc.want)
			}
		})
	}
}

// Re-running the bump with a prerelease name already used must fail rather
// than print a tag that exists.
func TestNextVersionRefusesAnExistingTag(t *testing.T) {
	if got, err := nextVersion(t, []string{"v26.09.0-alpha.1", "v26.09.0-alpha.2"}, "alpha.2"); err == nil {
		t.Fatalf("next-version.sh printed %q for a prerelease that is already tagged", got)
	}
}
