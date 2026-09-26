// Package capability resolves which host facilities ChairLift's pages and
// groups depend on, and answers whether this host provides them.
//
// ChairLift hides a group whose backing tool is absent rather than rendering
// it inert — the policy internal/autoupdate states in source. This package is
// the read-only, display-side classification that policy needs: Set.Supports
// has exactly the shape internal/navigation's VisibleItems already accepts as
// its `enabled` parameter, and Compose is how an administrator's group
// configuration is combined with it.
//
// Three rules shape everything here.
//
// A capability is the presence of a backing tool or asset, never a runtime
// state. That `bootc` is installed is a capability; whether this machine is
// booted from a bootc deployment is not, because answering it runs
// `bootc status`. Probes are restricted to the non-blocking operations —
// exec.LookPath, os.Stat, and environment reads — which is the constraint
// page-level resolution inherits when it is threaded through `buildUI`. A
// group whose gate needs a query rather than a presence check keeps its own
// asynchronous gate, builds a hidden shell, and is revealed once the query
// answers; it is deliberately left unclassified in the prerequisites table
// rather than given a capability that would answer a different question.
//
// Capability is a floor, not a ceiling. A group renders only when its
// configuration enables it and the host supports it, so configuration may
// subtract from the capability set and never add to it; Compose implements
// that direction, and a nil configuration predicate composes to false
// everywhere, which is the repository's fail-closed rule for a caller holding
// no configuration. Within the table the same rule holds — a group missing
// any capability it requires is unsupported — with one deliberate exception:
// an *unclassified* pair is reported as supported, so that a missing entry
// cannot silently hide a group at runtime. That exception is only safe because
// the table's totality is enforced against the configuration schema by a gate;
// see Prerequisites.
//
// The package is pure: it imports no puregotk, directly or transitively, so
// its table-driven tests run on a GTK-less host like every other decidable
// leaf under internal/ (ADR-0007).
package capability

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/projectbluefin/chairlift/internal/bootc"
	"github.com/projectbluefin/chairlift/internal/homebrew"
	"github.com/projectbluefin/chairlift/internal/imageinfo"
	"github.com/projectbluefin/chairlift/internal/ublue"
)

// Capability names one host facility a group's backing tool or asset
// provides. The string values are stable identifiers for diagnostics, not
// user-visible text.
type Capability string

const (
	// Flatpak is the flatpak command on $PATH.
	Flatpak Capability = "flatpak"
	// Homebrew is the brew command on $PATH. Its resolution is the
	// visibility half of the unified Homebrew path resolution; the execution
	// half stays with internal/homebrew.
	Homebrew Capability = "brew"
	// Distrobox is the distrobox command on $PATH.
	Distrobox Capability = "distrobox"
	// BootcStage is the installed OS staging script internal/bootc invokes
	// through pkexec. It is the prerequisite of the bootc updates group: a
	// host with the bootc command but no stage script has nothing for that
	// group to run.
	BootcStage Capability = "bootc-stage"
	// ImageDescriptor is the read-only ublue-os image descriptor. It is the
	// prerequisite of every Bluefin-family group, which has nothing to render
	// on a host that runs a different OS.
	ImageDescriptor Capability = "image-descriptor"
	// UblueHelper is the fixed-path privileged helper internal/ublue invokes
	// through pkexec (ublue.HelperPath). It is a separate capability from
	// the image descriptor because the two ship separately: Dakota carries
	// the descriptor and the bootc stage script but not this helper, so a
	// control that calls it would render and then fail. Only a regular file
	// counts — pkexec cannot run a directory.
	UblueHelper Capability = "ublue-helper"
)

// pathCapabilities pairs every capability provided by an executable with the
// name resolved on $PATH. Iterating one table keeps DetectWith's two probe
// kinds — one LookPath per binary, one Stat per asset — from drifting apart.
var pathCapabilities = []struct {
	capability Capability
	binary     string
}{
	{Flatpak, "flatpak"},
	{Distrobox, "distrobox"},
}

// assetCapabilities pairs every capability provided by a fixed path with the
// presence test that decides it. Each entry holds a whole predicate rather
// than a path list because Homebrew resolves through $PATH as well.
var assetCapabilities = []struct {
	capability Capability
	present    func(Probe) bool
}{
	{BootcStage, func(p Probe) bool { return exists(p, bootc.StageScriptPath) }},
	{ImageDescriptor, func(p Probe) bool { return exists(p, imageinfo.DescriptorPath) }},
	{UblueHelper, func(p Probe) bool { return regularFile(p, ublue.HelperPath) }},
	{Homebrew, func(p Probe) bool {
		if p.LookPath == nil || p.Stat == nil {
			return false
		}
		return homebrew.ResolveExecutable(p.LookPath, p.Stat) != ""
	}},
}

// providedCapabilities returns every capability either probe table can
// resolve, so a caller — and the tests — can enumerate the set from its
// single source of truth instead of restating it.
func providedCapabilities() []Capability {
	result := make([]Capability, 0, len(pathCapabilities)+len(assetCapabilities))
	for _, entry := range pathCapabilities {
		result = append(result, entry.capability)
	}
	for _, entry := range assetCapabilities {
		result = append(result, entry.capability)
	}
	return result
}

// Probe is the non-blocking host inspection surface. Both fields are function
// seams so resolution is testable without installing anything or touching the
// real host, and so the screenshot walkthrough can substitute a host shape
// under the chairlift_e2e build tag. There is no field for a command
// execution: a capability that needed one would be a runtime state, not a
// capability.
type Probe struct {
	// LookPath resolves a bare command name, as exec.LookPath does.
	LookPath func(name string) (string, error)
	// Stat reports one fixed path's existence, as os.Stat does.
	Stat func(name string) (os.FileInfo, error)
}

// systemProbe is the production probe: the two non-blocking operations the
// package is restricted to.
func systemProbe() Probe {
	return Probe{LookPath: exec.LookPath, Stat: os.Stat}
}

// probe is the injection seam for Detect, so a build that must render a host
// shape it is not running on can replace it before the first resolution.
var probe = systemProbe()

// SetProbe replaces the probe Detect resolves with. Call it before the window
// resolves capabilities, never after: page-level capabilities are resolved
// once during `buildUI` and stay immutable for the session, so a sidebar whose
// items shift under the user's cursor is not a state this package can produce.
func SetProbe(p Probe) {
	probe = p
}

// Set is a resolved capability inventory. The zero Set has no capabilities,
// which is the correct resolution for a host that has none of them and the
// fail-closed value for a caller that could not resolve.
type Set map[Capability]bool

// Has reports whether the host provides one capability.
func (s Set) Has(c Capability) bool {
	return s[c]
}

// Supports reports whether this host satisfies one page's group: every one
// of its AllOf capabilities, and one of its AnyOf alternatives when it lists
// any — see Prerequisites. A group the table does not classify requires
// nothing, so it is reported as supported.
//
// That last case is a runtime safety net rather than a licence to leave the
// table incomplete: internal/installcheck holds the table to
// config.SchemaGroups in both directions, so an unclassified group fails a
// gate rather than silently rendering on a host that cannot back it.
func (s Set) Supports(page, group string) bool {
	required, classified := requirementIndex[pageGroup{page, group}]
	if !classified {
		return true
	}
	return required.SatisfiedBy(s)
}

// Control names one helper-backed action rendered inside a group whose
// other controls do not share its prerequisite. Hiding the whole group
// would take working siblings with it — Powerwash beside Factory Reset,
// the stage button and published versions beside Roll Back — so the view
// asks SupportsControl for that one row instead.
type Control string

const (
	// RollbackControl is the Recovery page's Roll Back row, rendered
	// under bootc_updates_group beside the unprivileged Published
	// versions list. It calls chairlift-ublue-helper rollback.
	RollbackControl Control = "rollback"
	// FactoryResetControl is reset_group's Factory Reset row, beside
	// Powerwash, which needs no privilege. It calls chairlift-ublue-helper
	// factory-reset.
	FactoryResetControl Control = "factory-reset"
)

// ControlPrerequisite is one control, the group that renders it, and the
// capabilities it needs beyond that group's own.
type ControlPrerequisite struct {
	Control Control
	Page    string
	Group   string
	AllOf   []Capability
}

// controlPrerequisites classifies every Control constant.
// TestControlPrerequisitesSitInsideClassifiedGroups holds it to the
// constants and to the group table.
var controlPrerequisites = []ControlPrerequisite{
	{Control: FactoryResetControl, Page: "maintenance_page", Group: "reset_group", AllOf: []Capability{UblueHelper}},
	{Control: RollbackControl, Page: "updates_page", Group: "bootc_updates_group", AllOf: []Capability{UblueHelper}},
}

// SupportsControl reports whether this host has what one control needs
// beyond its group. The caller has already decided the group renders; this
// answers only for the row. An unclassified control is supported, for the
// same reason an unclassified group is. A nil Set supports no classified
// control, which is the fail-closed answer for a caller that could not
// resolve.
func (s Set) SupportsControl(c Control) bool {
	for _, entry := range controlPrerequisites {
		if entry.Control != c {
			continue
		}
		for _, required := range entry.AllOf {
			if !s[required] {
				return false
			}
		}
		return true
	}
	return true
}

// ControlPrerequisites returns every classified control, ordered by page,
// group, then control, each with a copy of its capabilities. The Help page
// walks it to explain which controls this host hides.
func ControlPrerequisites() []ControlPrerequisite {
	result := make([]ControlPrerequisite, len(controlPrerequisites))
	for i, entry := range controlPrerequisites {
		entry.AllOf = append([]Capability(nil), entry.AllOf...)
		result[i] = entry
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Page != result[j].Page {
			return result[i].Page < result[j].Page
		}
		if result[i].Group != result[j].Group {
			return result[i].Group < result[j].Group
		}
		return result[i].Control < result[j].Control
	})
	return result
}

// Compose composes an administrator's group configuration with a resolved
// capability set, producing the one predicate the sidebar navigation, the view
// builders, and the update coordinator are meant to share rather than each
// deriving its own answer. Capability is a floor: a group is enabled only when
// the configuration enables it and the host supports it, so configuration may
// subtract from the set and never add to it.
//
// A nil configured predicate composes to false for every pair. That matches
// navigation.VisibleItems' treatment of a nil predicate and keeps the
// repository's fail-closed rule: a caller with no configuration has no
// evidence that anything should render.
func Compose(configured func(page, group string) bool, set Set) func(page, group string) bool {
	return func(page, group string) bool {
		if configured == nil || !configured(page, group) {
			return false
		}
		return set.Supports(page, group)
	}
}

// Prerequisite is one classified page/group pair and the capabilities that
// can satisfy it: every one of AllOf, and at least one of AnyOf when AnyOf
// is non-empty. Both empty requires nothing of the host.
type Prerequisite struct {
	Page  string
	Group string
	AllOf []Capability
	AnyOf []Capability
}

// SatisfiedBy reports whether set backs this prerequisite.
func (p Prerequisite) SatisfiedBy(set Set) bool {
	for _, c := range p.AllOf {
		if !set[c] {
			return false
		}
	}
	if len(p.AnyOf) == 0 {
		return true
	}
	for _, c := range p.AnyOf {
		if set[c] {
			return true
		}
	}
	return false
}

// Gated reports whether the prerequisite names any capability, i.e. whether
// resolution can hide its group at all.
func (p Prerequisite) Gated() bool {
	return len(p.AllOf) > 0 || len(p.AnyOf) > 0
}

// Capabilities returns every capability the prerequisite names, AllOf first,
// as a fresh slice.
func (p Prerequisite) Capabilities() []Capability {
	result := make([]Capability, 0, len(p.AllOf)+len(p.AnyOf))
	result = append(result, p.AllOf...)
	return append(result, p.AnyOf...)
}

// copied returns p with its own copies of both capability slices.
func (p Prerequisite) copied() Prerequisite {
	p.AllOf = append([]Capability(nil), p.AllOf...)
	p.AnyOf = append([]Capability(nil), p.AnyOf...)
	return p
}

// prerequisites classifies every configurable group. The list is total over
// config.SchemaGroups, and internal/installcheck's
// TestCapabilityPrerequisitesMatchConfigSchema holds both directions, so a
// group added to the schema fails that gate until it is classified here.
//
// A group with no capabilities requires nothing of the host, and is listed
// anyway so that "no prerequisite" is a recorded decision rather than an
// omission. Two of those are worth naming, because they read like
// omissions:
//
//   - bootc_status_group renders whether this machine is booted from a bootc
//     deployment, which only `bootc status` can answer.
//   - features_group renders whether updex has features configured, which is
//     a query against the feature store rather than the presence of an asset.
//
// Those two keep their existing asynchronous gates in the view layer. Every
// other entry names the presences that make its group meaningful: all of
// AllOf, and any one of AnyOf.
//
// A group whose every control calls chairlift-ublue-helper needs
// UblueHelper in AllOf, so a host without it (Dakota) hides the group
// rather than rendering controls that fail. A group that mixes helper and
// non-helper controls does not: its helper-backed rows are floored one by
// one through controlPrerequisites instead.
var prerequisites = []Prerequisite{
	// Updates. automatic_updates_group's timer state is readable without
	// the helper, but the group is one switch, and flipping it is a helper
	// call; a read-only row is not what the group is for.
	{Page: "updates_page", Group: "automatic_updates_group", AllOf: []Capability{UblueHelper}},
	{Page: "updates_page", Group: "bootc_status_group"},
	{Page: "updates_page", Group: "bootc_updates_group", AnyOf: []Capability{BootcStage}},
	{Page: "updates_page", Group: "brew_trust_group", AnyOf: []Capability{Homebrew}},
	{Page: "updates_page", Group: "brew_updates_group", AnyOf: []Capability{Homebrew}},
	// The early-updates switch and the graphics-driver Switch are both
	// helper calls resolved against the descriptor.
	{Page: "updates_page", Group: "channel_group", AllOf: []Capability{ImageDescriptor, UblueHelper}},
	{Page: "updates_page", Group: "flatpak_updates_group", AnyOf: []Capability{Flatpak}},

	// Applications.
	{Page: "applications_page", Group: "applications_installed_group"},
	{Page: "applications_page", Group: "brew_bundles_group", AnyOf: []Capability{Homebrew}},
	{Page: "applications_page", Group: "brew_group", AnyOf: []Capability{Homebrew}},
	{Page: "applications_page", Group: "brew_search_group", AnyOf: []Capability{Homebrew}},
	{Page: "applications_page", Group: "flatpak_system_group", AnyOf: []Capability{Flatpak}},
	{Page: "applications_page", Group: "flatpak_user_group", AnyOf: []Capability{Flatpak}},

	// Agents.
	{Page: "agents_page", Group: "agents_group", AnyOf: []Capability{Homebrew}},

	// Features.
	{Page: "features_page", Group: "dx_group", AllOf: []Capability{ImageDescriptor, UblueHelper}},
	{Page: "features_page", Group: "features_group"},
	{Page: "features_page", Group: "gaming_group", AnyOf: []Capability{ImageDescriptor}},
	{Page: "help_page", Group: "troubleshooting_group", AnyOf: []Capability{Homebrew}},

	// Livery.
	{Page: "livery_page", Group: "account_group"},
	{Page: "livery_page", Group: "livery_app_grid_group"},
	{Page: "livery_page", Group: "livery_dock_group"},
	{Page: "livery_page", Group: "livery_foundation_group"},

	// Maintenance.
	{Page: "maintenance_page", Group: "maintenance_cleanup_group"},
	{Page: "maintenance_page", Group: "maintenance_freespace_group"},
	{Page: "maintenance_page", Group: "reset_group", AnyOf: []Capability{Flatpak, Distrobox}},

	// Help.
	{Page: "help_page", Group: "help_resources_group"},
}

type pageGroup struct{ page, group string }

// requirementIndex is prerequisites keyed by page and group, built once
// so a lookup does not scan the list per group per page build.
var requirementIndex = func() map[pageGroup]Prerequisite {
	index := make(map[pageGroup]Prerequisite, len(prerequisites))
	for _, p := range prerequisites {
		index[pageGroup{p.Page, p.Group}] = p
	}
	return index
}()

// Required returns the prerequisite of one page's group. classified is false
// for a pair the table does not classify.
//
// The returned slices are copies: a caller may sort or truncate them without
// changing the resolved table.
func Required(page, group string) (required Prerequisite, classified bool) {
	p, ok := requirementIndex[pageGroup{page, group}]
	if !ok {
		return Prerequisite{}, false
	}
	return p.copied(), true
}

// Prerequisites returns every classified pair, ordered by page and then group,
// each with a copy of its capabilities. A gate holds it to the configuration
// schema in both directions, and the Help page walks it to explain which
// configured groups this host cannot back.
func Prerequisites() []Prerequisite {
	result := make([]Prerequisite, len(prerequisites))
	for i, p := range prerequisites {
		result[i] = p.copied()
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Page != result[j].Page {
			return result[i].Page < result[j].Page
		}
		return result[i].Group < result[j].Group
	})
	return result
}

// assetPaths returns the fixed paths one asset capability's presence test
// reads. It mirrors assetCapabilities so an external host-shape stub can
// answer os.Stat for the same paths production inspects.
func assetPaths(c Capability) []string {
	switch c {
	case BootcStage:
		return []string{bootc.StageScriptPath}
	case ImageDescriptor:
		return []string{imageinfo.DescriptorPath}
	case UblueHelper:
		return []string{ublue.HelperPath}
	case Homebrew:
		return nil
	default:
		return nil
	}
}

// statFileInfo is the os.FileInfo returned by a stubbed Stat. Its zero mode
// reads as a regular file, which is all any presence test inspects.
type statFileInfo struct{ name string }

func (s statFileInfo) Name() string { return s.name }
func (s statFileInfo) Size() int64  { return 0 }
func (s statFileInfo) IsDir() bool  { return false }
func (s statFileInfo) ModTime() time.Time {
	return time.Time{}
}
func (s statFileInfo) Mode() os.FileMode { return 0 }
func (s statFileInfo) Sys() any          { return nil }

// ProbeFromPresent builds a Probe that reports exactly the named capabilities
// as present. It is the deterministic host-shape seam the screenshot
// walkthrough uses on a headless runner that ships none of these tools: the
// chairlift_e2e build substitutes one before the window resolves its
// capabilities, so the walkthrough renders a full Bluefin host rather than an
// empty one. It walks the same path and asset tables DetectWith does, so the
// stub can never resolve a capability that production would not.
func ProbeFromPresent(present ...Capability) Probe {
	presentSet := make(map[Capability]bool, len(present))
	for _, c := range present {
		presentSet[c] = true
	}

	paths := make(map[string]Capability, len(pathCapabilities)+1)
	paths["brew"] = Homebrew
	for _, entry := range pathCapabilities {
		paths[entry.binary] = entry.capability
	}
	for _, entry := range assetCapabilities {
		for _, path := range assetPaths(entry.capability) {
			paths[path] = entry.capability
		}
	}

	return Probe{
		LookPath: func(name string) (string, error) {
			if cap, ok := paths[name]; ok && presentSet[cap] {
				return name, nil
			}
			return "", fmt.Errorf("%q: not found", name)
		},
		Stat: func(name string) (os.FileInfo, error) {
			if cap, ok := paths[name]; ok && presentSet[cap] {
				return statFileInfo{name: name}, nil
			}
			return nil, fmt.Errorf("%q: no such file", name)
		},
	}
}

// ProbeFromNames builds a Probe from capability *names* — the string form of a
// Capability constant, e.g. "flatpak", "brew", "bootc-stage",
// "image-descriptor", "ublue-helper". It is the env-driven host-shape seam the
// screenshot walkthrough uses: the chairlift_e2e build splits a comma-separated
// environment variable and passes the words here. Names that are not a
// classified capability are ignored, so a typo or an unknown tool never
// resolves anything.
func ProbeFromNames(names ...string) Probe {
	present := make([]Capability, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		present = append(present, Capability(name))
	}
	return ProbeFromPresent(present...)
}

// Detect resolves this host's capability set through the package probe. Call
// it once during window construction and keep the result: capabilities are
// resolved once per session and never re-probed, so sidebar accelerators and
// items do not shift under the user's cursor.
//
// Every probe is a LookPath or a Stat, so the call is cheap enough to run on
// the GTK main thread. Nothing here is cached: the immutability is the
// caller's contract, and a package-level cache would make a test's probe
// substitution order-dependent.
func Detect() Set {
	return DetectWith(probe)
}

// DetectWith resolves a capability set through p. It is the pure core Detect
// wraps, and what the tests drive directly.
func DetectWith(p Probe) Set {
	set := make(Set, len(pathCapabilities)+len(assetCapabilities))
	for _, entry := range pathCapabilities {
		set[entry.capability] = onPath(p, entry.binary)
	}

	// The OS staging and helper capabilities are asset presences, not
	// command presences: each group's action is a fixed path invoked through
	// pkexec, so that path is what the group needs.
	for _, entry := range assetCapabilities {
		set[entry.capability] = entry.present(p)
	}
	return set
}

// onPath reports whether one command resolves on $PATH. A probe that returns
// an error for any reason — absent, or not executable — is not present.
func onPath(p Probe, name string) bool {
	if p.LookPath == nil {
		return false
	}
	_, err := p.LookPath(name)
	return err == nil
}

// exists reports whether one fixed path is present. A probe that returns an
// error for any reason is not present.
func exists(p Probe, path string) bool {
	if p.Stat == nil {
		return false
	}
	_, err := p.Stat(path)
	return err == nil
}

// regularFile reports whether one fixed path is present as a regular file,
// following symlinks as os.Stat does. A directory or device at the path is
// not a program pkexec can run.
func regularFile(p Probe, path string) bool {
	if p.Stat == nil {
		return false
	}
	info, err := p.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
