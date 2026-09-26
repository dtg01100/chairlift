package pageview

import (
	"testing"

	"github.com/projectbluefin/chairlift/internal/capability"
)

func onlyGroups(pairs ...[2]string) func(page, group string) bool {
	return func(page, group string) bool {
		for _, p := range pairs {
			if p == [2]string{page, group} {
				return true
			}
		}
		return false
	}
}

func allGroups(string, string) bool { return true }

func TestUnavailableFeaturesListsConfiguredGroupsTheHostCannotBack(t *testing.T) {
	configured := onlyGroups(
		[2]string{"updates_page", "flatpak_updates_group"},
		[2]string{"maintenance_page", "reset_group"},
		[2]string{"agents_page", "agents_group"},
	)
	// Homebrew present backs Agent Mode; Distrobox alone backs Recovery.
	set := capability.Set{capability.Homebrew: true}

	got := UnavailableFeatures(set, configured)
	want := []Row{
		{Title: "Recovery", Subtitle: "Needs Flatpak or Distrobox"},
		{Title: "App updates", Subtitle: "Needs Flatpak"},
	}
	if len(got) != len(want) {
		t.Fatalf("UnavailableFeatures() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestUnavailableFeaturesOmitsConfigDisabledGroups(t *testing.T) {
	// Nothing on the host, but configuration enables nothing either: every
	// hidden group is the administrator's choice, not a missing tool.
	if got := UnavailableFeatures(capability.Set{}, func(string, string) bool { return false }); len(got) != 0 {
		t.Errorf("config-disabled groups listed: %+v", got)
	}
	if got := UnavailableFeatures(capability.Set{}, nil); len(got) != 0 {
		t.Errorf("nil configuration listed groups: %+v", got)
	}
}

func TestUnavailableFeaturesIsEmptyWhenEveryCapabilityIsPresent(t *testing.T) {
	set := capability.Set{}
	for _, p := range capability.Prerequisites() {
		for _, c := range p.Capabilities() {
			set[c] = true
		}
	}
	for _, p := range capability.ControlPrerequisites() {
		for _, c := range p.AllOf {
			set[c] = true
		}
	}
	if got := UnavailableFeatures(set, allGroups); len(got) != 0 {
		t.Errorf("fully capable host listed: %+v", got)
	}
}

// dakotaShape is every capability a Dakota host has: everything but the
// ChairLift ublue helper, which Dakota does not ship.
func dakotaShape() capability.Set {
	set := capability.Set{}
	for _, p := range capability.Prerequisites() {
		for _, c := range p.Capabilities() {
			set[c] = true
		}
	}
	set[capability.UblueHelper] = false
	return set
}

const needsHelper = "Needs the Control Center system helper (/usr/bin/chairlift-ublue-helper)"

// TestUnavailableFeaturesNamesTheMissingHelper is the Dakota case: every
// control that would call chairlift-ublue-helper is hidden, and Help must
// say that the helper is what is missing — for whole groups and for the
// single controls hidden inside groups that still render.
func TestUnavailableFeaturesNamesTheMissingHelper(t *testing.T) {
	got := UnavailableFeatures(dakotaShape(), allGroups)
	want := []Row{
		{Title: "Developer mode", Subtitle: needsHelper},
		{Title: "Automatic updates", Subtitle: needsHelper},
		{Title: "Release channel", Subtitle: needsHelper},
		{Title: "Factory Reset", Subtitle: needsHelper},
		{Title: "Roll Back", Subtitle: needsHelper},
	}
	if len(got) != len(want) {
		t.Fatalf("UnavailableFeatures() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A group that needs two things names only what is actually missing, and
// names both when both are.
func TestUnavailableFeaturesNamesOnlyWhatIsMissing(t *testing.T) {
	configured := onlyGroups([2]string{"features_page", "dx_group"})

	withHelper := capability.Set{capability.UblueHelper: true}
	if got := UnavailableFeatures(withHelper, configured); len(got) != 1 ||
		got[0] != (Row{Title: "Developer mode", Subtitle: "Needs /usr/share/ublue-os/image-info.json"}) {
		t.Errorf("helper present, descriptor missing: %+v", got)
	}

	nothing := UnavailableFeatures(capability.Set{}, configured)
	want := Row{
		Title:    "Developer mode",
		Subtitle: "Needs /usr/share/ublue-os/image-info.json and the Control Center system helper (/usr/bin/chairlift-ublue-helper)",
	}
	if len(nothing) != 1 || nothing[0] != want {
		t.Errorf("both missing: %+v, want [%+v]", nothing, want)
	}
}

// A hidden control is explained only when its own group renders: when the
// group is disabled by the administrator the control is not missing, and
// when the host cannot back the group its own row already explains it.
func TestUnavailableFeaturesExplainsControlsOnlyInsideRenderedGroups(t *testing.T) {
	configured := onlyGroups([2]string{"updates_page", "bootc_updates_group"})
	got := UnavailableFeatures(dakotaShape(), configured)
	if len(got) != 1 || got[0] != (Row{Title: "Roll Back", Subtitle: needsHelper}) {
		t.Errorf("rollback only: %+v", got)
	}

	// reset_group configured but unbacked: the group row, no control row.
	resetOnly := onlyGroups([2]string{"maintenance_page", "reset_group"})
	got = UnavailableFeatures(capability.Set{}, resetOnly)
	if len(got) != 1 || got[0].Title != "Recovery" {
		t.Errorf("unbacked reset group: %+v, want the Recovery row alone", got)
	}
}

func TestEveryCapabilityGatedGroupHasATitle(t *testing.T) {
	// With nothing present every gated group is listed, so a table gap shows
	// up as a blank title or an unnamed capability.
	rows := UnavailableFeatures(capability.Set{}, allGroups)
	if len(rows) == 0 {
		t.Fatal("no capability-gated groups; the check is vacuous")
	}
	for _, p := range capability.Prerequisites() {
		if !p.Gated() {
			continue
		}
		if featureTitles[[2]string{p.Page, p.Group}] == "" {
			t.Errorf("%s/%s has no title", p.Page, p.Group)
		}
		for _, c := range p.Capabilities() {
			if capabilityNames[c] == "" {
				t.Errorf("capability %q has no name", c)
			}
		}
	}
	for key := range featureTitles {
		if required, ok := capability.Required(key[0], key[1]); !ok || !required.Gated() {
			t.Errorf("%s/%s is titled but not capability-gated", key[0], key[1])
		}
	}
	for _, p := range capability.ControlPrerequisites() {
		if controlTitles[p.Control] == "" {
			t.Errorf("control %q has no title", p.Control)
		}
		for _, c := range p.AllOf {
			if capabilityNames[c] == "" {
				t.Errorf("capability %q has no name", c)
			}
		}
	}
}
