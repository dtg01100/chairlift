package actionstate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdatesPageUsesGuardedRefreshDecisions(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	path := filepath.Join(repoRoot, "internal", "views", "updates_page.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(source)

	// The guard here is the gate/generation/row bookkeeping, not the button
	// copy: label text and its typography belong to the HIG pass, so the
	// control-state assertions below are deliberately copy-agnostic.
	for _, required := range []string{
		`if !updateGate.TryStart()`,
		`go uh.updateHomebrew(btn, updateGate)`,
		`if !upgradeGate.TryStart()`,
		`dryRun := dryrun.Enabled()`,
		`actionstate.PackageUpgrade(err == nil, dryRun)`,
		`actionstate.OutdatedRefresh(err == nil, currentCount, len(packages))`,
		`actionstate.OutdatedPresentation(refresh.Count)`,
		`uh.outdatedRows.Remove(row`,
		`uh.updateCounts.Add(badgestate.Homebrew, -1)`,
		`actionstate.OutdatedPresentation(remaining)`,
		`uh.loadOutdatedPackages()`,
		`actionstate.MetadataUpdate(err == nil, dryRun)`,
		`uh.loadOutdatedPackagesWithDone(func(bool)`,
		`generation := uh.brewRefresh.Begin()`,
		`go uh.loadOutdatedPackagesGeneration(generation, done)`,
		`!uh.brewRefresh.IsCurrent(generation)`,
		`uh.outdatedRows.Clear(func(row *adw.ActionRow)`,
		`uh.outdatedRows.Add(row)`,
	} {
		if !strings.Contains(text, required) {
			t.Errorf("updates-page wiring does not contain %q", required)
		}
	}
}

func TestFeaturesPageDeveloperModeUsesGate(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))

	viewsPath := filepath.Join(repoRoot, "internal", "views", "views.go")
	viewsSource, err := os.ReadFile(viewsPath)
	if err != nil {
		t.Fatalf("read %s: %v", viewsPath, err)
	}
	viewsText := string(viewsSource)
	if !strings.Contains(viewsText, "developerGate") || !strings.Contains(viewsText, "actionstate.Gate") {
		t.Errorf("views.go UserHome does not contain developerGate actionstate.Gate field")
	}

	featuresPath := filepath.Join(repoRoot, "internal", "views", "features_page.go")
	featuresSource, err := os.ReadFile(featuresPath)
	if err != nil {
		t.Fatalf("read %s: %v", featuresPath, err)
	}
	featuresText := string(featuresSource)

	for _, required := range []string{
		`uh.developerGate.TryStart()`,
		`uh.developerGate.Reset()`,
		// The recursion guard used to be a bare `toggle.SetState(` beside
		// every `SetActive`, which only held while every call site
		// remembered to pair them. guardedSwitch owns that pairing —
		// newGuardedSwitch sets both, and set() re-enters behind an
		// `applying` flag the handler checks — so the property is now
		// asserted where it is enforced rather than at each call site.
		`toggle.set(`,
		`newGuardedSwitch(`,
	} {
		if !strings.Contains(featuresText, required) {
			t.Errorf("features_page wiring does not contain %q", required)
		}
	}
}

// A button that is made sensitive again after a run must release its gate;
// Complete permanently rejects every future click, including retries after
// a failed or dry-run action. Views cannot be imported by headless tests.
func TestRepeatableControlsReleaseTheirGates(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	viewsDir := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
	for file, gates := range map[string][]string{
		"updates_page.go": {"driverGate"},
		"reset.go":        {"powerwashGate", "factoryResetGate"},
		"agents_page.go":  {"agentPresetGate"},
		// The developer switch and the optional feed setup behind it are
		// both repeatable: the switch is used again after every toggle, and
		// the setup gate has to reopen when its worker finishes or a second
		// enable could never install anything.
		"features_page.go": {"developerGate", "developerFeedGate"},
	} {
		data, err := os.ReadFile(filepath.Join(viewsDir, file))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, gate := range gates {
			if !strings.Contains(text, "uh."+gate+".TryStart()") || !strings.Contains(text, "uh."+gate+".Reset()") || strings.Contains(text, "uh."+gate+".Complete()") {
				t.Errorf("%s: repeatable %s must start and reset, never complete", file, gate)
			}
		}
	}
}

// bootc rollback toggles the selected deployment. A successful live click
// must close its gate; only a failed attempt or dry-run preview may retry.
func TestRollbackGateCompletesOnlyAfterLiveSuccess(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "recovery.go"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "func (uh *UserHome) onBootcRollbackClicked()", 2)
	if len(parts) != 2 {
		t.Fatal("rollback handler not found")
	}
	body := parts[1]
	for _, required := range []string{"uh.bootcRollbackGate.TryStart()", "if decision.Confirm {", "uh.bootcRollbackGate.Complete()", "button.SetSensitive(false)", "uh.bootcRollbackGate.Reset()", "button.SetSensitive(true)"} {
		if !strings.Contains(body, required) {
			t.Errorf("rollback must remain one-shot on live success but retry after failure or preview: missing %q", required)
		}
	}
}

// TestLiveryToggleDerivesDryRunBehaviorFromOneDecision pins ADR-0009 rule 3
// for the Livery page's section master switches: the dry-run flag is read
// once per handler, turned into an actionmsg.LiveryToggleDecision, and that
// one value drives the in-memory mirror, the sub-rows' sensitivity, the
// switch restore in releaseLiveryToggle, and the preview toast. The page
// builder itself cannot be imported on a headless host (ADR-0007), so the
// wiring is asserted against the source text.
func TestLiveryToggleDerivesDryRunBehaviorFromOneDecision(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate wiring_test.go")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "livery_actions.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)

	for _, required := range []string{
		`"github.com/projectbluefin/chairlift/internal/dryrun"`,
		`"github.com/projectbluefin/chairlift/internal/views/actionmsg"`,
		`actionmsg.LiveryToggle(dryrun.Enabled(), enabled, pageview.LiverySectionName(livery.AppGrid))`,
		`actionmsg.LiveryToggle(dryrun.Enabled(), enabled, pageview.LiverySectionName(surface))`,
		`defer uh.releaseLiveryToggle(livery.AppGrid, decision)`,
		`defer uh.releaseLiveryToggle(surface, decision)`,
		`func (uh *UserHome) releaseLiveryToggle(s livery.Surface, decision actionmsg.LiveryToggleDecision) {`,
	} {
		if !strings.Contains(text, required) {
			t.Errorf("livery_actions.go wiring does not contain %q", required)
		}
	}

	// gofmt keeps both blocks at the same indentation, so the snippets are
	// matched verbatim: the branch has to sit immediately above the
	// assignment, otherwise a regression that moves the assignment out from
	// under its guard would still pass a "builds a decision" check. The
	// app-grid branch also has to wrap the row's SetSensitive call and the
	// surface branch has to wrap setLiverySectionSensitive — both are part
	// of the visible-state advance under --dry-run (#424 follow-up).
	appGridMirror := "\tif decision.MutateUI {\n" +
		"\t\tuh.liveryState.AppGridEnabled = enabled\n" +
		"\t\tif uh.liveryAppGridRow != nil {\n" +
		"\t\t\tuh.liveryAppGridRow.SetSensitive(enabled)\n" +
		"\t\t}\n" +
		"\t}\n"
	surfaceMirror := "\tif decision.MutateUI {\n" +
		"\t\tuh.setLiveryToggleState(surface, enabled)\n" +
		"\t\tuh.setLiverySectionSensitive(surface, enabled)\n" +
		"\t}\n"
	releaseRestore := "\t\tif !decision.MutateUI {\n" +
		"\t\t\tuh.restoreLiveryToggleWidget(s, toggle)\n" +
		"\t\t}\n"
	previewToast := "\t\tif decision.Toast != \"\" {\n" +
		"\t\t\tuh.toastAdder.ShowToast(decision.Toast)\n" +
		"\t\t}\n"
	for name, snippet := range map[string]string{
		"app-grid mirror":   appGridMirror,
		"panel/dock mirror": surfaceMirror,
		"switch restore":    releaseRestore,
		"preview toast":     previewToast,
	} {
		if !strings.Contains(text, snippet) {
			t.Errorf("livery_actions.go %s does not branch on the single LiveryToggleDecision; expected the contiguous block:\n%s", name, snippet)
		}
	}

	// ADR-0009 rule 3: the flag is read once per handler and nowhere else.
	// A third read — the one releaseLiveryToggle used to make to decide
	// whether to snap the switch back — is exactly the drift the decision
	// struct exists to prevent, so the count is an upper bound, not a floor.
	if got := strings.Count(text, "dryrun.Enabled()"); got != 2 {
		t.Errorf("livery_actions.go reads dryrun.Enabled() %d times, want exactly 2 (one per toggle handler; every other site must derive from the decision)", got)
	}
	if strings.Contains(text, "if !dryrun.Enabled() {") || strings.Contains(text, "if dryrun.Enabled() {") {
		t.Error("livery_actions.go still branches directly on dryrun.Enabled(); mutation and toast must both derive from actionmsg.LiveryToggle")
	}
}
