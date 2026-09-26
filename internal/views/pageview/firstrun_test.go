package pageview

import (
	"strings"
	"testing"

	"github.com/projectbluefin/chairlift/internal/branding"
	"github.com/projectbluefin/chairlift/internal/firstrun"
	"github.com/projectbluefin/chairlift/internal/updateflow"
)

func TestWelcomeViewModelConstructsProperDefaults(t *testing.T) {
	vm := NewWelcomeViewModel(false)

	if vm.Title != WelcomeTitle {
		t.Errorf("Title = %q, want %q", vm.Title, WelcomeTitle)
	}
	if vm.Subtitle != WelcomeSubtitle {
		t.Errorf("Subtitle = %q, want %q", vm.Subtitle, WelcomeSubtitle)
	}
	if vm.PrimaryButtonText != ConfigureEverythingAction {
		t.Errorf("PrimaryButtonText = %q, want %q", vm.PrimaryButtonText, ConfigureEverythingAction)
	}
	if vm.SecondaryButtonText != GetMovingAction {
		t.Errorf("SecondaryButtonText = %q, want %q", vm.SecondaryButtonText, GetMovingAction)
	}
	if !vm.DefaultIsPrimary {
		t.Error("DefaultIsPrimary should be true for recommended flow")
	}
	if vm.WordmarkAsset != firstrun.AssetWordmarkLight {
		t.Errorf("WordmarkAsset for light theme = %q, want %q", vm.WordmarkAsset, firstrun.AssetWordmarkLight)
	}
}

func TestWelcomeViewModelThemeWordmarkSelection(t *testing.T) {
	lightVM := NewWelcomeViewModel(false)
	if lightVM.WordmarkAsset != firstrun.AssetWordmarkLight {
		t.Errorf("light wordmark = %q, want %q", lightVM.WordmarkAsset, firstrun.AssetWordmarkLight)
	}

	darkVM := NewWelcomeViewModel(true)
	if darkVM.WordmarkAsset != firstrun.AssetWordmarkDark {
		t.Errorf("dark wordmark = %q, want %q", darkVM.WordmarkAsset, firstrun.AssetWordmarkDark)
	}
}

func TestGetMovingToastMessageContainsAppNameAndCommand(t *testing.T) {
	msg := GetMovingToastMessage()
	if !strings.Contains(msg, branding.AppName) {
		t.Errorf("toast message %q does not contain AppName %q", msg, branding.AppName)
	}
	if !strings.Contains(msg, "chairlift") {
		t.Errorf("toast message %q does not mention chairlift command", msg)
	}
	// Issue #265 specifies "by running `chairlift`." — a space before the
	// period reads as a typo in a toast the user cannot dismiss and reread.
	if strings.Contains(msg, " .") {
		t.Errorf("toast message %q has a stray space before the period", msg)
	}
	if !strings.Contains(msg, "Application Menu") {
		t.Errorf("toast message %q does not mention Application Menu", msg)
	}
}

func TestWelcomeActionHeadingsAndSubtitlesAreNonEmpty(t *testing.T) {
	if WelcomeTitle == "" {
		t.Error("WelcomeTitle must not be empty")
	}
	if WelcomeSubtitle == "" {
		t.Error("WelcomeSubtitle must not be empty")
	}
	if ConfigureEverythingAction == "" {
		t.Error("ConfigureEverythingAction must not be empty")
	}
	if ConfigureEverythingDescription == "" {
		t.Error("ConfigureEverythingDescription must not be empty")
	}
	if GetMovingAction == "" {
		t.Error("GetMovingAction must not be empty")
	}
	if GetMovingDescription() == "" {
		t.Error("GetMovingDescription must not be empty")
	}
	if ConfigStepInfoSubtitle() == "" {
		t.Error("ConfigStepInfoSubtitle must not be empty")
	}
}

// TestFirstRunCopyNamesTheProductThroughBranding holds the branding package's
// ownership of the user-facing product name: a literal here drifts silently on
// rename, because installcheck's display-name gate only covers the code name.
func TestFirstRunCopyNamesTheProductThroughBranding(t *testing.T) {
	for name, text := range map[string]string{
		"GetMovingDescription":   GetMovingDescription(),
		"ConfigStepInfoSubtitle": ConfigStepInfoSubtitle(),
		"GetMovingToastMessage":  GetMovingToastMessage(),
	} {
		if !strings.Contains(text, branding.AppName) {
			t.Errorf("%s() = %q, does not name branding.AppName %q", name, text, branding.AppName)
		}
	}

	for name, format := range map[string]string{
		"GetMovingDescriptionFormat":   GetMovingDescriptionFormat,
		"ConfigStepInfoSubtitleFormat": ConfigStepInfoSubtitleFormat,
		"GetMovingToastFormat":         GetMovingToastFormat,
	} {
		if !strings.Contains(format, "%s") {
			t.Errorf("%s = %q, must interpolate the product name", name, format)
		}
	}
}

// TestStepForwardActionNamesWhatTheClickDoes covers the label the forward
// button carries: "Finish" on an intermediate step promises a completion the
// click does not deliver, because the assistant shows the next step instead.
func TestStepForwardActionNamesWhatTheClickDoes(t *testing.T) {
	if got := StepForwardAction(false); got != NextStepAction {
		t.Errorf("StepForwardAction(false) = %q, want %q", got, NextStepAction)
	}
	if got := StepForwardAction(true); got != FinishAction {
		t.Errorf("StepForwardAction(true) = %q, want %q", got, FinishAction)
	}
}

func TestNavigationAndCompletionCopyIsNonEmpty(t *testing.T) {
	for name, text := range map[string]string{
		"BackAction":            BackAction,
		"NextStepAction":        NextStepAction,
		"FinishAction":          FinishAction,
		"SetupCompletedMessage": SetupCompletedMessage,
	} {
		if text == "" {
			t.Errorf("%s must not be empty", name)
		}
	}
}

// TestWelcomeCopyIsReExportedFromFirstrun keeps the welcome screen's copy
// owned by one package; a restated literal here drifts from the step.
func TestWelcomeCopyIsReExportedFromFirstrun(t *testing.T) {
	if WelcomeTitle != firstrun.StepWelcome.Title {
		t.Errorf("WelcomeTitle = %q, want %q", WelcomeTitle, firstrun.StepWelcome.Title)
	}
	if WelcomeSubtitle != firstrun.StepWelcome.Description {
		t.Errorf("WelcomeSubtitle = %q, want %q", WelcomeSubtitle, firstrun.StepWelcome.Description)
	}
}

// TestEveryModelChoiceHasARowOrIsTheCollectionList holds the dialog to the
// model: every choice the setup model can offer either resolves to one
// switch row's copy here, or is the Apps choice, which the dialog renders
// as one row per discovered collection. A choice added to the model without
// a row here would render as an empty title.
func TestEveryModelChoiceHasARowOrIsTheCollectionList(t *testing.T) {
	model := firstrun.NewAssistantModel(func(string, string) bool { return true })
	seen := 0
	for _, step := range model.Steps() {
		for _, choice := range step.Choices {
			seen++
			row, ok := SetupChoiceRow(choice.ID)
			if choice.ID == firstrun.ChoiceIDBundles {
				if ok {
					t.Errorf("SetupChoiceRow(%q) = %+v, want no single row for the collection list", choice.ID, row)
				}
				continue
			}
			if !ok || row.Title == "" {
				t.Errorf("SetupChoiceRow(%q) = (%+v, %v), want a titled row", choice.ID, row, ok)
			}
		}
	}
	if seen < 8 {
		t.Fatalf("model offered %d choices with an open floor, want the full inventory", seen)
	}
	if _, ok := SetupChoiceRow("no-such-choice"); ok {
		t.Error("SetupChoiceRow(unknown) reported a row")
	}
}

// TestAppearanceChoicesReuseTheLiveryRows keeps one voice per setting: the
// assistant's Appearance rows are the Livery page's own switch rows. The
// app-grid row alone carries one more sentence, because the assistant offers
// its switch without the brand chooser the Livery page has.
func TestAppearanceChoicesReuseTheLiveryRows(t *testing.T) {
	for choiceID, want := range map[string]Row{
		firstrun.ChoiceIDFoundation: LiveryPanelRow(),
		firstrun.ChoiceIDDock:       LiveryDockRow(),
	} {
		if got, _ := SetupChoiceRow(choiceID); got != want {
			t.Errorf("SetupChoiceRow(%q) = %+v, want the Livery row %+v", choiceID, got, want)
		}
	}
	got, _ := SetupChoiceRow(firstrun.ChoiceIDAppGrid)
	page := LiveryAppGridRow()
	if got.Title != page.Title || !strings.HasPrefix(got.Subtitle, page.Subtitle) || !strings.HasSuffix(got.Subtitle, SetupAppGridBrandHint) {
		t.Errorf("SetupChoiceRow(app-grid) = %+v, want the Livery row %+v followed by %q", got, page, SetupAppGridBrandHint)
	}
}

// TestUpdateChoicesAreSpelledAsTheSourceTableKeys binds the model's Update
// Preferences choices to the one source table both the Preferences dialog
// and the assistant render: the dialog binds a switch to a GSettings key by
// the choice's ID alone, so a mismatch would leave a switch unbound.
func TestUpdateChoicesAreSpelledAsTheSourceTableKeys(t *testing.T) {
	model := firstrun.NewAssistantModel(func(string, string) bool { return true })
	var keys []string
	for _, step := range model.Steps() {
		if step.ID != firstrun.StepIDUpdates {
			continue
		}
		for _, choice := range step.Choices {
			preference, ok := UpdateSourcePreferenceByKey(choice.ID)
			if !ok {
				t.Errorf("update choice %q is not a key in UpdateSourcePreferences", choice.ID)
				continue
			}
			if preference.Title != choice.Title {
				t.Errorf("choice %q titled %q, the source table says %q", choice.ID, choice.Title, preference.Title)
			}
			keys = append(keys, choice.ID)
		}
	}
	if len(keys) != len(UpdateSourcePreferences) {
		t.Fatalf("the Update Preferences step offers %d sources, the table has %d", len(keys), len(UpdateSourcePreferences))
	}
}

// TestUpdateSourcePreferenceRowsExplainWhyASwitchIsLocked is the availability
// rule the Preferences dialog and the assistant share: a switch is operable
// only once the shell has checked, and only for a source the administrator
// enables and the host can back — and the subtitle says which.
func TestUpdateSourcePreferenceRowsExplainWhyASwitchIsLocked(t *testing.T) {
	id := updateflow.Applications
	cases := []struct {
		name          string
		states        []updateflow.SourceState
		ready         bool
		wantSubtitle  string
		wantSensitive bool
	}{
		{"before the first check", nil, false, "Checking availability…", false},
		{"a source the shell has not reported", []updateflow.SourceState{{ID: updateflow.OperatingSystem, Configured: true, Available: true}}, true, "Checking availability…", false},
		{"disabled by the administrator", []updateflow.SourceState{{ID: id, Configured: false, Available: true}}, true, "Disabled by your administrator", false},
		{"not backed by this host", []updateflow.SourceState{{ID: id, Configured: true, Available: false}}, true, "Not available on this system", false},
		{"operable", []updateflow.SourceState{{ID: id, Configured: true, Available: true}}, true, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := UpdateSourcePreferenceSubtitle(tc.states, tc.ready, id); got != tc.wantSubtitle {
				t.Errorf("subtitle = %q, want %q", got, tc.wantSubtitle)
			}
			if got := UpdateSourcePreferenceSensitive(tc.states, tc.ready, id); got != tc.wantSensitive {
				t.Errorf("sensitive = %v, want %v", got, tc.wantSensitive)
			}
		})
	}
}

// TestConfigureEverythingDescriptionNamesTheStepsItLeadsTo: the tooltip on
// the primary action promised "developer tools" while the third step is
// Update Preferences; the copy must name what the steps actually are.
func TestConfigureEverythingDescriptionNamesTheStepsItLeadsTo(t *testing.T) {
	model := firstrun.NewAssistantModel(func(string, string) bool { return true })
	for _, step := range model.Steps() {
		if step.ID == firstrun.StepIDWelcome {
			continue
		}
		if !strings.Contains(strings.ToLower(ConfigureEverythingDescription), strings.ToLower(step.Title)) {
			t.Errorf("ConfigureEverythingDescription %q does not name the %q step", ConfigureEverythingDescription, step.Title)
		}
	}
}
