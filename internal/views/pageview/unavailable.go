package pageview

import (
	"strings"

	"github.com/projectbluefin/chairlift/internal/bootc"
	"github.com/projectbluefin/chairlift/internal/branding"
	"github.com/projectbluefin/chairlift/internal/capability"
	"github.com/projectbluefin/chairlift/internal/imageinfo"
	"github.com/projectbluefin/chairlift/internal/ublue"
)

// featureTitles names every group the capability floor can hide. Groups with
// no prerequisite are never hidden by it, so they have no entry;
// TestEveryCapabilityGatedGroupHasATitle holds the table total over
// capability.Prerequisites.
var featureTitles = map[[2]string]string{
	{"updates_page", "automatic_updates_group"}:   "Automatic updates",
	{"updates_page", "bootc_updates_group"}:       "System updates",
	{"updates_page", "flatpak_updates_group"}:     "App updates",
	{"updates_page", "brew_updates_group"}:        "Developer tool updates",
	{"updates_page", "brew_trust_group"}:          "Unverified Homebrew sources",
	{"updates_page", "channel_group"}:             "Release channel",
	{"applications_page", "flatpak_user_group"}:   "Your apps",
	{"applications_page", "flatpak_system_group"}: "Shared apps",
	{"applications_page", "brew_group"}:           "Packages from Homebrew",
	{"applications_page", "brew_search_group"}:    "Find more apps and tools",
	{"applications_page", "brew_bundles_group"}:   "App collections",
	{"agents_page", "agents_group"}:               "Agent Mode",
	{"features_page", "dx_group"}:                 "Developer mode",
	{"features_page", "gaming_group"}:             "Gaming mode",
	{"help_page", "troubleshooting_group"}:        "Enhanced Troubleshooting",
	{"maintenance_page", "reset_group"}:           "Recovery",
}

// controlTitles names every single control the capability floor can hide
// inside a group that still renders. TestEveryCapabilityGatedGroupHasATitle
// holds it total over capability.ControlPrerequisites.
var controlTitles = map[capability.Control]string{
	capability.RollbackControl:     "Roll Back",
	capability.FactoryResetControl: "Factory Reset",
}

// capabilityNames is what a person would look for on their system to supply
// one capability. Technical names are fine here: the rows sit behind an
// expander the user opened to ask why. The helper gets words before its path,
// because "chairlift-ublue-helper" means nothing to someone who knows the
// application as Control Center.
var capabilityNames = map[capability.Capability]string{
	capability.Flatpak:         "Flatpak",
	capability.Homebrew:        "Homebrew",
	capability.Distrobox:       "Distrobox",
	capability.BootcStage:      bootc.StageScriptPath,
	capability.ImageDescriptor: imageinfo.DescriptorPath,
	capability.UblueHelper:     "the " + branding.AppName + " system helper (" + ublue.HelperPath + ")",
}

// UnavailableFeatures lists the groups configuration enables but the host
// capability set hides, then the single controls it hides inside groups that
// still render, each titled for a person and subtitled with what is missing.
// It consumes the already-resolved set, so explaining never re-probes. A nil
// configured predicate lists nothing: a group configuration disabled is the
// administrator's choice, not a missing tool.
func UnavailableFeatures(set capability.Set, configured func(page, group string) bool) []Row {
	if configured == nil {
		return nil
	}
	var rows []Row
	for _, p := range capability.Prerequisites() {
		if !configured(p.Page, p.Group) || set.Supports(p.Page, p.Group) {
			continue
		}
		rows = append(rows, Row{
			Title:    featureTitles[[2]string{p.Page, p.Group}],
			Subtitle: needs(set, p.AllOf, p.AnyOf),
		})
	}
	// A control is only missing from a group that is on screen: when the
	// group itself is hidden, its own row above already says why.
	for _, p := range capability.ControlPrerequisites() {
		if !configured(p.Page, p.Group) || !set.Supports(p.Page, p.Group) || set.SupportsControl(p.Control) {
			continue
		}
		rows = append(rows, Row{
			Title:    controlTitles[p.Control],
			Subtitle: needs(set, p.AllOf, nil),
		})
	}
	return rows
}

// needs words what is missing: each absent capability the feature requires,
// and the alternatives when none of them is present, joined with "and".
func needs(set capability.Set, allOf, anyOf []capability.Capability) string {
	var missing []string
	for _, c := range allOf {
		if !set.Has(c) {
			missing = append(missing, capabilityNames[c])
		}
	}
	if len(anyOf) > 0 {
		alternatives := make([]string, 0, len(anyOf))
		satisfied := false
		for _, c := range anyOf {
			satisfied = satisfied || set.Has(c)
			alternatives = append(alternatives, capabilityNames[c])
		}
		if !satisfied {
			missing = append(missing, strings.Join(alternatives, " or "))
		}
	}
	return "Needs " + strings.Join(missing, " and ")
}
