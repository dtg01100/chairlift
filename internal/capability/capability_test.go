package capability

import (
	"errors"
	"os"
	"testing"

	"github.com/projectbluefin/chairlift/internal/bootc"
	"github.com/projectbluefin/chairlift/internal/homebrew"
	"github.com/projectbluefin/chairlift/internal/imageinfo"
	"github.com/projectbluefin/chairlift/internal/ublue"
)

// Every test in this file drives DetectWith with a fake Probe, so the suite
// resolves a described host rather than the machine it runs on. The package
// imports no puregotk, so the file also runs on a GTK-less host — which is
// itself the executable check that ADR-0007's purity constraint holds: a
// puregotk import anywhere in the dependency graph would panic at package
// init here, before any test function ran.

// fakeHost describes a host as the two answers the probes consult: which bare
// names resolve on $PATH, and which fixed paths exist. Asset paths are spelled
// with the owning package's own constant, so a fixture cannot drift from the
// path production probes.
func fakeHost(binaries, assets []string) Probe {
	onPath := make(map[string]bool, len(binaries))
	for _, binary := range binaries {
		onPath[binary] = true
	}
	present := make(map[string]bool, len(assets))
	for _, asset := range assets {
		present[asset] = true
	}

	return Probe{
		LookPath: func(name string) (string, error) {
			if onPath[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("executable file not found in $PATH")
		},
		// Stat checks path presence and regular file attributes.
		Stat: func(name string) (os.FileInfo, error) {
			if present[name] {
				return statFileInfo{name: name}, nil
			}
			return nil, os.ErrNotExist
		},
	}
}

// allBinaries and allAssets are the complete probe answers for a host that has
// everything.
var (
	allBinaries = []string{"flatpak", "distrobox"}
	allAssets   = []string{
		bootc.StageScriptPath,
		imageinfo.DescriptorPath,
		homebrew.FallbackExecutable(),
		ublue.HelperPath,
	}
)

// fullSet is what DetectWith resolves on a host that has every capability.
func fullSet() Set {
	set := Set{}
	for _, c := range providedCapabilities() {
		set[c] = true
	}
	return set
}

// TestDetectWithResolvesEveryCapability covers each capability's probe, the
// two asset capabilities that need more than one path, and the fail-closed
// answer for a probe that cannot answer at all.
func TestDetectWithResolvesEveryCapability(t *testing.T) {
	tests := []struct {
		name  string
		probe Probe
		want  Set
	}{
		{
			name:  "no tooling at all",
			probe: fakeHost(nil, nil),
			want:  Set{},
		},
		{
			name:  "flatpak on PATH",
			probe: fakeHost([]string{"flatpak"}, nil),
			want:  Set{Flatpak: true},
		},
		{
			name:  "brew on PATH",
			probe: fakeHost([]string{"brew"}, nil),
			want:  Set{Homebrew: true},
		},
		{
			name:  "distrobox on PATH",
			probe: fakeHost([]string{"distrobox"}, nil),
			want:  Set{Distrobox: true},
		},
		{
			name:  "bootc stage script installed",
			probe: fakeHost(nil, []string{bootc.StageScriptPath}),
			want:  Set{BootcStage: true},
		},
		{
			name:  "ublue image descriptor present",
			probe: fakeHost(nil, []string{imageinfo.DescriptorPath}),
			want:  Set{ImageDescriptor: true},
		},
		{
			name:  "ublue privileged helper installed",
			probe: fakeHost(nil, []string{ublue.HelperPath}),
			want:  Set{UblueHelper: true},
		},
		{
			// A directory at the fixed path is not a helper pkexec can
			// run; only a regular file counts.
			name: "ublue privileged helper path is a directory",
			probe: Probe{
				LookPath: fakeHost(nil, nil).LookPath,
				Stat: func(name string) (os.FileInfo, error) {
					if name == ublue.HelperPath {
						return dirFileInfo{statFileInfo{name: name}}, nil
					}
					return nil, os.ErrNotExist
				},
			},
			want: Set{},
		},
		{
			name:  "homebrew on PATH",
			probe: fakeHost([]string{"brew"}, nil),
			want:  Set{Homebrew: true},
		},
		{
			name:  "homebrew at fallback path without brew on PATH",
			probe: fakeHost(nil, []string{homebrew.FallbackExecutable()}),
			want:  Set{Homebrew: true},
		},
		{
			name:  "every capability present",
			probe: fakeHost(allBinaries, allAssets),
			want:  fullSet(),
		},
		{
			// A nil probe function cannot answer, and cannot answer is not
			// present.
			name:  "unset probe functions resolve nothing",
			probe: Probe{},
			want:  Set{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectWith(tt.probe)

			// Compare across the complete capability set rather than the
			// expected one, so a probe that wrongly resolves something is a
			// failure too.
			capabilities := providedCapabilities()
			if len(capabilities) == 0 {
				t.Fatal("providedCapabilities() is empty: the comparison would be vacuous")
			}
			for _, c := range capabilities {
				if got.Has(c) != tt.want.Has(c) {
					t.Errorf("DetectWith(...).Has(%q) = %v, want %v", c, got.Has(c), tt.want.Has(c))
				}
			}
		})
	}
}

// TestProbeFromPresentAndNamesIsTheE2EHostShapeSeam locks the deterministic
// host-shape seam the screenshot walkthrough uses: ProbeFromPresent builds a
// Probe from capability values, ProbeFromNames from their string form, and
// both must resolve exactly the named capabilities through the same path and
// asset tables DetectWith uses. ProbeFromNames also ignores unknown names,
// so a typo in CHAIRLIFT_CAPABILITIES never resolves anything production
// would not. This is the regression test that catches the two helpers
// drifting apart from production resolution.
func TestProbeFromPresentAndNamesIsTheE2EHostShapeSeam(t *testing.T) {
	// A representative Bluefin host shape: flatpak, brew, the bootc
	// stage script, the image descriptor, and the ublue helper.
	want := Set{
		Flatpak:         true,
		Homebrew:        true,
		BootcStage:      true,
		ImageDescriptor: true,
		UblueHelper:     true,
	}

	present := ProbeFromPresent(Flatpak, Homebrew, BootcStage, ImageDescriptor, UblueHelper)
	names := ProbeFromNames("flatpak", "brew", "bootc-stage", "image-descriptor", "ublue-helper")

	for _, probe := range []Probe{present, names} {
		got := DetectWith(probe)
		capabilities := providedCapabilities()
		for _, c := range capabilities {
			if got.Has(c) != want.Has(c) {
				t.Errorf("ProbeFromPresent/Names Has(%q) = %v, want %v", c, got.Has(c), want.Has(c))
			}
		}
	}

	// ProbeFromNames ignores unknown and empty names rather than resolving
	// anything production would not.
	ignoring := DetectWith(ProbeFromNames("flatpak", "not-a-capability", ""))
	if !ignoring.Has(Flatpak) {
		t.Errorf("ProbeFromNames dropped flatpak despite unknown names: %#v", ignoring)
	}
	for _, c := range providedCapabilities() {
		if c != Flatpak && ignoring.Has(c) {
			t.Errorf("ProbeFromNames resolved unexpected %q: %#v", c, ignoring)
		}
	}
}

// TestEveryRequiredCapabilityHasAProbe holds the prerequisites table to the
// two probe tables: a capability named by a group but resolvable by no probe
// would make that group permanently hidden on every host, because a
// Capability is a string and a typo in the table still compiles.
func TestEveryRequiredCapabilityHasAProbe(t *testing.T) {
	provided := make(map[Capability]bool)
	for _, c := range providedCapabilities() {
		provided[c] = true
	}
	if len(provided) == 0 {
		t.Fatal("providedCapabilities() is empty: the gate would be vacuous")
	}

	checked := 0
	for _, entry := range Prerequisites() {
		for _, c := range entry.Capabilities() {
			checked++
			if !provided[c] {
				t.Errorf(
					"prerequisite %s/%s requires %q, which no probe resolves",
					entry.Page, entry.Group, c,
				)
			}
		}
	}
	for _, entry := range ControlPrerequisites() {
		for _, c := range entry.AllOf {
			checked++
			if !provided[c] {
				t.Errorf("control %q requires %q, which no probe resolves", entry.Control, c)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no prerequisite names a capability: the gate would be vacuous")
	}
}

// TestSupportsAnyOneCapabilityOfItsGroup derives its cases from the
// prerequisites table, so every entry — including each capability of a
// multi-capability group — is covered without restating the table here. A
// group needs every one of its AllOf capabilities and, when it lists any,
// one of its AnyOf alternatives.
func TestSupportsAnyOneCapabilityOfItsGroup(t *testing.T) {
	entries := Prerequisites()
	if len(entries) == 0 {
		t.Fatal("Prerequisites() is empty: the gate would be vacuous")
	}

	gated := 0
	for _, entry := range entries {
		t.Run(entry.Page+"/"+entry.Group, func(t *testing.T) {
			if !entry.Gated() {
				// No host prerequisite: resolution cannot hide it.
				if !(Set{}).Supports(entry.Page, entry.Group) {
					t.Errorf(
						"Supports(%q, %q) = false on a host with no capabilities, want true: the group has no prerequisite",
						entry.Page, entry.Group,
					)
				}
				return
			}

			gated++

			if (Set{}).Supports(entry.Page, entry.Group) {
				t.Errorf(
					"Supports(%q, %q) = true on a host with no capabilities, want false",
					entry.Page, entry.Group,
				)
			}

			base := func() Set {
				set := Set{}
				for _, c := range entry.AllOf {
					set[c] = true
				}
				return set
			}

			if len(entry.AnyOf) == 0 {
				if !base().Supports(entry.Page, entry.Group) {
					t.Errorf("Supports(%q, %q) = false with every AllOf capability present, want true", entry.Page, entry.Group)
				}
			} else if len(entry.AllOf) > 0 && base().Supports(entry.Page, entry.Group) {
				t.Errorf("Supports(%q, %q) = true with no AnyOf alternative present, want false", entry.Page, entry.Group)
			}

			// Any single one of the group's alternatives is enough, beside
			// the capabilities it always needs.
			for _, c := range entry.AnyOf {
				set := base()
				set[c] = true
				if !set.Supports(entry.Page, entry.Group) {
					t.Errorf(
						"Supports(%q, %q) = false with %q present, want true",
						entry.Page, entry.Group, c,
					)
				}
			}

			// Every AllOf capability is necessary: a fully capable host
			// missing only that one cannot back the group.
			for _, c := range entry.AllOf {
				set := fullSet()
				set[c] = false
				if set.Supports(entry.Page, entry.Group) {
					t.Errorf(
						"Supports(%q, %q) = true without %q, want false",
						entry.Page, entry.Group, c,
					)
				}
			}

			// Everything else is not: a group is satisfied only by its own
			// capabilities.
			others := Set{}
			for _, c := range providedCapabilities() {
				if !containsCapability(entry.Capabilities(), c) {
					others[c] = true
				}
			}
			if others.Supports(entry.Page, entry.Group) {
				t.Errorf(
					"Supports(%q, %q) = true with only unrelated capabilities present, want false",
					entry.Page, entry.Group,
				)
			}
		})
	}

	if gated == 0 {
		t.Fatal("no prerequisite names a capability: the gate would be vacuous")
	}
}

// TestHelperActionGroupsNeedTheUblueHelper pins the groups whose every
// control calls chairlift-ublue-helper, and the siblings that must survive
// without it. Dakota ships the image descriptor, the stage script, Flatpak,
// and Homebrew, but not the helper: on that host a helper-only group must be
// hidden rather than rendered and then failed.
func TestHelperActionGroupsNeedTheUblueHelper(t *testing.T) {
	withoutHelper := fullSet()
	withoutHelper[UblueHelper] = false

	for _, pair := range [][2]string{
		{"updates_page", "automatic_updates_group"},
		{"updates_page", "channel_group"},
		{"features_page", "dx_group"},
	} {
		if withoutHelper.Supports(pair[0], pair[1]) {
			t.Errorf("Supports(%q, %q) = true without the ublue helper, want false", pair[0], pair[1])
		}
		if !fullSet().Supports(pair[0], pair[1]) {
			t.Errorf("Supports(%q, %q) = false on a fully capable host, want true", pair[0], pair[1])
		}
	}

	// The image descriptor is still required where it was: the helper alone
	// on a non-ublue host has no channel or developer groups to act on.
	withoutDescriptor := fullSet()
	withoutDescriptor[ImageDescriptor] = false
	for _, pair := range [][2]string{{"updates_page", "channel_group"}, {"features_page", "dx_group"}} {
		if withoutDescriptor.Supports(pair[0], pair[1]) {
			t.Errorf("Supports(%q, %q) = true without the image descriptor, want false", pair[0], pair[1])
		}
	}

	// Mixed groups keep rendering: their helper-backed controls are
	// floored individually through SupportsControl.
	for _, pair := range [][2]string{
		{"updates_page", "bootc_updates_group"},
		{"maintenance_page", "reset_group"},
		{"features_page", "gaming_group"},
	} {
		if !withoutHelper.Supports(pair[0], pair[1]) {
			t.Errorf("Supports(%q, %q) = false without the ublue helper, want true: only a control inside it needs the helper", pair[0], pair[1])
		}
	}
}

// TestSupportsControlFloorsHelperActionsInsideMixedGroups covers the
// control-level floor: Roll Back and Factory Reset share a group with
// controls that need no helper, so only they disappear.
func TestSupportsControlFloorsHelperActionsInsideMixedGroups(t *testing.T) {
	withoutHelper := fullSet()
	withoutHelper[UblueHelper] = false

	for _, control := range []Control{RollbackControl, FactoryResetControl} {
		if withoutHelper.SupportsControl(control) {
			t.Errorf("SupportsControl(%q) = true without the ublue helper, want false", control)
		}
		if !(Set{UblueHelper: true}).SupportsControl(control) {
			t.Errorf("SupportsControl(%q) = false with the ublue helper, want true", control)
		}
		if (Set(nil)).SupportsControl(control) {
			t.Errorf("SupportsControl(%q) = true on a nil set, want false", control)
		}
	}

	// An undeclared control is unclassified, and like an unclassified group
	// it is not silently hidden.
	if !(Set{}).SupportsControl(Control("not-a-control")) {
		t.Error("SupportsControl(unknown) = false, want true for an unclassified control")
	}
}

// TestControlPrerequisitesSitInsideClassifiedGroups holds the control table
// to the group table: a control names the group that renders it, and that
// group must be classified and not already require what the control does —
// otherwise the control entry is dead weight.
func TestControlPrerequisitesSitInsideClassifiedGroups(t *testing.T) {
	entries := ControlPrerequisites()
	if len(entries) == 0 {
		t.Fatal("ControlPrerequisites() is empty: the gate would be vacuous")
	}
	seen := make(map[Control]bool)
	for _, entry := range entries {
		if seen[entry.Control] {
			t.Errorf("control %q is listed twice", entry.Control)
		}
		seen[entry.Control] = true
		group, classified := Required(entry.Page, entry.Group)
		if !classified {
			t.Errorf("control %q names unclassified group %s/%s", entry.Control, entry.Page, entry.Group)
			continue
		}
		if len(entry.AllOf) == 0 {
			t.Errorf("control %q requires nothing; it needs no entry", entry.Control)
		}
		for _, c := range entry.AllOf {
			if containsCapability(group.AllOf, c) {
				t.Errorf("control %q requires %q, which its group %s/%s already requires", entry.Control, c, entry.Page, entry.Group)
			}
		}
	}
	for _, control := range []Control{RollbackControl, FactoryResetControl} {
		if !seen[control] {
			t.Errorf("control %q has no ControlPrerequisites entry", control)
		}
	}
}

// TestSupportsTreatsUnclassifiedGroupAsSupported covers the runtime safety
// net. The table's totality over config.SchemaGroups is held by
// internal/installcheck; this case fixes what happens if a pair reaches
// Supports anyway, which is that configuration alone decides rather than a
// missing entry silently hiding a group.
func TestSupportsTreatsUnclassifiedGroupAsSupported(t *testing.T) {
	tests := []struct {
		name  string
		page  string
		group string
	}{
		{name: "unknown page", page: "not_a_page", group: "brew_group"},
		{name: "unknown group on a known page", page: "features_page", group: "not_a_group"},
		{name: "empty names", page: "", group: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			empty := Set{}
			if !empty.Supports(tt.page, tt.group) {
				t.Errorf("Supports(%q, %q) = false, want true for an unclassified pair", tt.page, tt.group)
			}
			if !fullSet().Supports(tt.page, tt.group) {
				t.Errorf("Supports(%q, %q) = false on a fully capable host, want true", tt.page, tt.group)
			}
		})
	}

	if _, classified := Required("not_a_page", "brew_group"); classified {
		t.Error("Required reported an unknown page as classified")
	}
}

// TestComposeRequiresConfigurationAndCapability pins the floor's direction:
// configuration may subtract from the capability set and never add to it.
func TestComposeRequiresConfigurationAndCapability(t *testing.T) {
	configured := func(page, group string) bool {
		return page == "agents_page" && group == "agents_group"
	}

	tests := []struct {
		name       string
		page       string
		group      string
		configured func(page, group string) bool
		set        Set
		want       bool
	}{
		{
			name:       "configured and capable",
			page:       "agents_page",
			group:      "agents_group",
			configured: configured,
			set:        Set{Homebrew: true},
			want:       true,
		},
		{
			name:       "configured but incapable",
			page:       "agents_page",
			group:      "agents_group",
			configured: configured,
			set:        Set{},
			want:       false,
		},
		{
			name:       "capable but not configured",
			page:       "applications_page",
			group:      "brew_group",
			configured: configured,
			set:        fullSet(),
			want:       false,
		},
		{
			name:       "nil configuration fails closed even where the host is capable",
			page:       "agents_page",
			group:      "agents_group",
			configured: nil,
			set:        fullSet(),
			want:       false,
		},
		{
			name:       "nil configuration fails closed for an unclassified pair too",
			page:       "not_a_page",
			group:      "not_a_group",
			configured: nil,
			set:        fullSet(),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := Compose(tt.configured, tt.set)
			if got := predicate(tt.page, tt.group); got != tt.want {
				t.Errorf("Compose(...)(%q, %q) = %v, want %v", tt.page, tt.group, got, tt.want)
			}
		})
	}
}

// TestRequiredReturnsACopy makes the doc comment's promise executable: a
// caller that sorts or truncates what Required hands back must not be able to
// change how the next call resolves.
func TestRequiredReturnsACopy(t *testing.T) {
	entry := firstGatedPrerequisite(t)

	required, classified := Required(entry.Page, entry.Group)
	if !classified {
		t.Fatalf("Required(%q, %q) reported the table entry as unclassified", entry.Page, entry.Group)
	}
	if len(required.Capabilities()) != len(entry.Capabilities()) {
		t.Fatalf("Required(%q, %q) returned %d capabilities, want %d", entry.Page, entry.Group, len(required.Capabilities()), len(entry.Capabilities()))
	}

	for _, list := range [][]Capability{required.AllOf, required.AnyOf} {
		for i := range list {
			list[i] = Capability("clobbered")
		}
	}
	again, _ := Required(entry.Page, entry.Group)
	if containsCapability(again.Capabilities(), Capability("clobbered")) {
		t.Error("mutating the slices Required returned changed the resolved table")
	}
}

// TestPrerequisitesAreOrderedUniquelyAndCopied holds the collection the
// installcheck totality gate consumes: it must be deterministic, free of
// duplicate pairs, and safe for a caller to mutate.
func TestPrerequisitesAreOrderedUniquelyAndCopied(t *testing.T) {
	entries := Prerequisites()
	if len(entries) == 0 {
		t.Fatal("Prerequisites() is empty: the gate would be vacuous")
	}

	seen := make(map[string]bool, len(entries))
	for i, entry := range entries {
		pair := entry.Page + "/" + entry.Group
		if seen[pair] {
			t.Errorf("Prerequisites() names %s twice", pair)
		}
		seen[pair] = true

		if i > 0 {
			previous := entries[i-1]
			if previous.Page > entry.Page ||
				(previous.Page == entry.Page && previous.Group > entry.Group) {
				t.Errorf("Prerequisites() is out of order at %s", pair)
			}
		}
	}

	// Truncating a copy must not shrink the table's own entry.
	for i, entry := range entries {
		if len(entry.AnyOf) == 0 {
			continue
		}
		want := entry.AnyOf[0]
		entries[i].AnyOf = entries[i].AnyOf[:0]
		again, _ := Required(entry.Page, entry.Group)
		if len(again.AnyOf) == 0 || again.AnyOf[0] != want {
			t.Fatalf("truncating the slice Prerequisites returned changed %s/%s", entry.Page, entry.Group)
		}
		break
	}
}

// TestDetectUsesThePackageProbe covers the SetProbe seam the screenshot
// walkthrough's capability stub replaces. It restores the production probe
// immediately after, so no other test observes it.
func TestDetectUsesThePackageProbe(t *testing.T) {
	t.Cleanup(func() { probe = systemProbe() })

	SetProbe(fakeHost([]string{"flatpak"}, nil))

	if got := Detect(); !got.Has(Flatpak) || got.Has(Homebrew) {
		t.Errorf("Detect() = %v, want flatpak only from the substituted probe", got)
	}
}

// firstGatedPrerequisite returns the first table entry that names a
// capability, failing the test if the table has none.
func firstGatedPrerequisite(t *testing.T) Prerequisite {
	t.Helper()

	for _, entry := range Prerequisites() {
		if entry.Gated() {
			return entry
		}
	}
	t.Fatal("no prerequisite names a capability: the test would be vacuous")
	return Prerequisite{}
}

// containsCapability reports whether a capability list names c.
func containsCapability(capabilities []Capability, c Capability) bool {
	for _, candidate := range capabilities {
		if candidate == c {
			return true
		}
	}
	return false
}

// dirFileInfo reports a directory, so a probe can place one at a path a
// capability expects to be a regular file.
type dirFileInfo struct{ statFileInfo }

func (dirFileInfo) IsDir() bool       { return true }
func (dirFileInfo) Mode() os.FileMode { return os.ModeDir | 0o755 }
