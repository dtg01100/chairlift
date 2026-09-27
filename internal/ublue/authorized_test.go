package ublue

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/projectbluefin/chairlift/internal/dryrun"
	"github.com/projectbluefin/chairlift/internal/imageinfo"
	"github.com/projectbluefin/chairlift/internal/ubluehelper"
)

// dakotaInfo is a Bluefin-family descriptor for the Detect cases.
var dakotaInfo = imageinfo.Info{Name: "dakota", Tag: "latest", Ref: "docker://ghcr.io/projectbluefin/dakota"}

// regularFile is a FileInfo for an installed helper binary.
type regularFile struct{}

func (regularFile) Name() string       { return filepath.Base(HelperPath) }
func (regularFile) Size() int64        { return 1 }
func (regularFile) Mode() os.FileMode  { return 0o755 }
func (regularFile) ModTime() time.Time { return time.Time{} }
func (regularFile) IsDir() bool        { return false }
func (regularFile) Sys() any           { return nil }

// stubHost points the policy scan at dir and reports the helper as installed
// or absent.
func stubHost(t *testing.T, dir string, helperInstalled bool) {
	t.Helper()
	previousStat, previousDir := statHelper, policyActionsDir
	statHelper = func(string) (os.FileInfo, error) {
		if helperInstalled {
			return regularFile{}, nil
		}
		return nil, os.ErrNotExist
	}
	policyActionsDir = dir
	t.Cleanup(func() { statHelper, policyActionsDir = previousStat, previousDir })
}

func writePolicy(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The policy ChairLift ships must authorize every command its helper
// accepts. If the two drift, an image built from this release would hide a
// control whose helper command works, or offer one pkexec refuses.
func TestShippedPolicyAuthorizesEverySupportedCommand(t *testing.T) {
	dir := t.TempDir()
	policy, err := os.ReadFile(filepath.Join("..", "..", "data", "io.projectbluefin.chairlift.ublue.policy"))
	if err != nil {
		t.Fatal(err)
	}
	writePolicy(t, dir, "io.projectbluefin.chairlift.ublue.policy", string(policy))
	stubHost(t, dir, true)

	got := sortedKeys(authorizedCommands())
	want := ubluehelper.SupportedCommands()
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authorized commands = %v, want every supported command %v", got, want)
	}
}

// An image that ships an older policy authorizes only what that policy
// names; a command it lacks must not be offered, however new the GUI.
func TestAuthorizedCommandsAreThoseTheInstalledPolicyNames(t *testing.T) {
	dir := t.TempDir()
	writePolicy(t, dir, "older.policy", `<policyconfig>
  <action id="a.restart">
    <annotate key="org.freedesktop.policykit.exec.path">`+HelperPath+`</annotate>
    <annotate key="org.freedesktop.policykit.exec.argv1">`+ubluehelper.CommandRestart+`</annotate>
  </action>
  <action id="a.rollback">
    <annotate key="org.freedesktop.policykit.exec.path">`+HelperPath+`</annotate>
    <annotate key="org.freedesktop.policykit.exec.argv1">`+ubluehelper.CommandRollback+`</annotate>
  </action>
  <action id="a.other-binary">
    <annotate key="org.freedesktop.policykit.exec.path">/usr/bin/something-else</annotate>
    <annotate key="org.freedesktop.policykit.exec.argv1">`+ubluehelper.CommandFactoryReset+`</annotate>
  </action>
  <action id="a.unknown-command">
    <annotate key="org.freedesktop.policykit.exec.path">`+HelperPath+`</annotate>
    <annotate key="org.freedesktop.policykit.exec.argv1">not-a-helper-command</annotate>
  </action>
</policyconfig>`)
	writePolicy(t, dir, "broken.policy", "<policyconfig><action>")
	writePolicy(t, dir, "notes.txt", "not a policy")
	stubHost(t, dir, true)

	got := sortedKeys(authorizedCommands())
	want := []string{ubluehelper.CommandRestart, ubluehelper.CommandRollback}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authorized commands = %v, want %v", got, want)
	}
}

// An installed policy is not enough: without the helper binary pkexec has
// nothing to run.
func TestAuthorizedCommandsRequireTheHelperBinary(t *testing.T) {
	dir := t.TempDir()
	policy, err := os.ReadFile(filepath.Join("..", "..", "data", "io.projectbluefin.chairlift.ublue.policy"))
	if err != nil {
		t.Fatal(err)
	}
	writePolicy(t, dir, "io.projectbluefin.chairlift.ublue.policy", string(policy))
	stubHost(t, dir, false)

	if got := authorizedCommands(); got != nil {
		t.Errorf("authorized commands = %v, want none without the helper binary", got)
	}
}

func TestAuthorizedCommandsWithoutPolicyDirectory(t *testing.T) {
	stubHost(t, filepath.Join(t.TempDir(), "missing"), true)

	if got := authorizedCommands(); got != nil {
		t.Errorf("authorized commands = %v, want none without an actions directory", got)
	}
}

// A dry run never invokes the helper, so it previews every control even on an
// image that has not shipped one.
func TestDetectOffersEveryCommandInDryRun(t *testing.T) {
	stubDetection(t, dakotaInfo, nil, nil)
	previous := dryrun.Enabled()
	dryrun.Set(true)
	t.Cleanup(func() { dryrun.Set(previous) })

	status, err := Detect()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Supports(ubluehelper.SupportedCommands()...) {
		t.Errorf("dry-run Detect commands = %v, want every supported command", sortedKeys(status.Commands))
	}
}

// Outside a dry run, a Bluefin host whose image ships no helper offers none
// of its controls, while keeping the unprivileged status the views render.
func TestDetectOffersNoCommandWithoutTheHelper(t *testing.T) {
	stubDetection(t, dakotaInfo, nil, nil)
	previous := dryrun.Enabled()
	dryrun.Set(false)
	t.Cleanup(func() { dryrun.Set(previous) })

	status, err := Detect()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available {
		t.Fatal("Detect() available = false, want the descriptor still reported")
	}
	for _, command := range ubluehelper.SupportedCommands() {
		if status.Supports(command) {
			t.Errorf("Supports(%q) = true on an image without the helper", command)
		}
	}
}

func TestSupportsRequiresEveryCommand(t *testing.T) {
	status := Status{Commands: map[string]bool{
		ubluehelper.CommandDXEnable: true,
	}}

	if !status.Supports(ubluehelper.CommandDXEnable) {
		t.Error("Supports(dx-enable) = false, want true")
	}
	if status.Supports(ubluehelper.CommandDXEnable, ubluehelper.CommandDXDisable) {
		t.Error("Supports(dx-enable, dx-disable) = true with dx-disable missing, want false")
	}
	if status.Supports() {
		t.Error("Supports() with no commands = true, want false")
	}
}
