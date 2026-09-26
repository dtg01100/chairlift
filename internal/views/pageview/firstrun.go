package pageview

import (
	"fmt"

	"github.com/projectbluefin/chairlift/internal/branding"
	"github.com/projectbluefin/chairlift/internal/firstrun"
	"github.com/projectbluefin/chairlift/internal/updateflow"
)

const (
	// WelcomeTitle is the prominent header on the onboarding hero screen.
	// The welcome step and the hero screen are one screen, so the copy is
	// owned by internal/firstrun and re-exported here.
	WelcomeTitle = firstrun.WelcomeStepTitle

	// WelcomeSubtitle is the welcoming narrative copy explaining the system's nature.
	WelcomeSubtitle = firstrun.WelcomeStepDescription

	// ConfigureEverythingAction is the label for the recommended default action.
	ConfigureEverythingAction = "Configure Everything"

	// ConfigureEverythingDescription describes proceeding into optional decision steps.
	ConfigureEverythingDescription = "Proceed into optional onboarding steps to customize your appearance, apps, and update preferences."

	// GetMovingAction is the label for immediately exiting the wizard to the desktop.
	GetMovingAction = "Get Moving"

	// GetMovingDescriptionFormat is the template describing the skip action;
	// the product name is interpolated from branding.AppName.
	GetMovingDescriptionFormat = "Jump straight to your desktop. You can open %s anytime from the Application Menu or terminal."

	// ConfigStepInfoTitle heads the reassurance row on a configuration step.
	ConfigStepInfoTitle = "Onboarding Configuration"

	// ConfigStepInfoSubtitleFormat is the template reassuring the user the
	// choice is not final; the product name comes from branding.AppName.
	ConfigStepInfoSubtitleFormat = "Every choice here can be changed later in %s."

	// SetupChoicesLoadingSubtitle is the subtitle a setup step shows while the
	// page that owns its controls has not finished reading their state.
	SetupChoicesLoadingSubtitle = "Checking the current setting…"

	// SetupChoiceUnavailableSubtitle is the subtitle for an Appearance choice
	// the desktop cannot apply, such as the panel mark without its extension.
	SetupChoiceUnavailableSubtitle = "Not available on this desktop"

	// SetupChoiceBusyMessage is the toast shown when a setup switch could not
	// be applied because the owning page was still loading or busy.
	SetupChoiceBusyMessage = "That setting is still loading. Try again in a moment."

	// SetupBundlesLoadingTitle is the placeholder row while collections are
	// still being discovered for the Apps step.
	SetupBundlesLoadingTitle = "Looking for app collections…"

	// SetupAppGridBrandHint follows the app-grid row's subtitle in the setup
	// assistant, which offers the switch but not the brand chooser.
	SetupAppGridBrandHint = "Pick the brand on the Livery page"

	// WordmarkAccessibleName is what a screen reader announces for the hero
	// screen's wordmark picture, which otherwise has no text at all.
	WordmarkAccessibleName = "Project Bluefin"

	// GetMovingToastFormat is the template for the reassuring exit toast.
	GetMovingToastFormat = "You're ready to go! You can launch %s anytime from the Application Menu or by running chairlift."

	// BackAction labels the configuration step's reverse navigation button.
	BackAction = "Back"

	// NextStepAction labels the forward button while configuration steps remain.
	NextStepAction = "Next"

	// FinishAction labels the forward button on the final configuration step.
	FinishAction = "Finish"

	// SetupCompletedMessage is the toast confirming the assistant finished.
	SetupCompletedMessage = "Setup completed!"
)

// StepForwardAction names the forward navigation button for a configuration
// step. finishes reports whether clicking it concludes setup rather than
// moving onto a further step.
//
// The button advances through the remaining steps before it finishes setup,
// so labeling it "Finish" on an intermediate step misdescribes the click the
// user is about to make.
func StepForwardAction(finishes bool) string {
	if finishes {
		return FinishAction
	}
	return NextStepAction
}

// GetMovingDescription describes skipping the wizard while affirming the
// application remains available afterwards.
func GetMovingDescription() string {
	return fmt.Sprintf(GetMovingDescriptionFormat, branding.AppName)
}

// ConfigStepInfoSubtitle returns the reassurance subtitle for a configuration step.
func ConfigStepInfoSubtitle() string {
	return fmt.Sprintf(ConfigStepInfoSubtitleFormat, branding.AppName)
}

// GetMovingToastMessage returns the affirming status toast text.
func GetMovingToastMessage() string {
	return fmt.Sprintf(GetMovingToastFormat, branding.AppName)
}

// SetupChoiceRow is the row text for one setup choice, keyed on the
// firstrun.Choice ID. The Appearance choices reuse the Livery page's own
// switch rows, and the Update Preferences choices reuse the Preferences
// dialog's source titles, so a setting is described in one voice on both
// surfaces. ok is false for a choice that has no single row of its own (the
// Apps step lists one row per collection instead).
//
// The app-grid mark is the one choice that needs a second decision the
// assistant does not offer — which brand — so its row says where that is
// made; the panel and Files marks come with a default and need nothing else.
func SetupChoiceRow(choiceID string) (row Row, ok bool) {
	switch choiceID {
	case firstrun.ChoiceIDAppGrid:
		row = LiveryAppGridRow()
		row.Subtitle += ". " + SetupAppGridBrandHint
		return row, true
	case firstrun.ChoiceIDFoundation:
		return LiveryPanelRow(), true
	case firstrun.ChoiceIDDock:
		return LiveryDockRow(), true
	}
	if preference, found := UpdateSourcePreferenceByKey(choiceID); found {
		return Row{Title: preference.Title}, true
	}
	return Row{}, false
}

// UpdateSourcePreference pairs one update source with the
// io.projectbluefin.chairlift.updates key recording whether the user includes
// it, and the title both the Preferences dialog and the setup assistant show.
type UpdateSourcePreference struct {
	ID    updateflow.SourceID
	Key   string
	Title string
}

// UpdateSourcePreferences is the one table of user-selectable update sources,
// in the order the Preferences dialog lists them.
var UpdateSourcePreferences = []UpdateSourcePreference{
	{ID: updateflow.OperatingSystem, Key: "operating-system-enabled", Title: "Operating system"},
	{ID: updateflow.Applications, Key: "applications-enabled", Title: "Applications"},
	{ID: updateflow.DeveloperTools, Key: "developer-tools-enabled", Title: "Developer tools"},
	{ID: updateflow.SystemComponents, Key: "system-components-enabled", Title: "System components"},
}

// UpdateSourcePreferenceByKey resolves a settings key to its source.
func UpdateSourcePreferenceByKey(key string) (UpdateSourcePreference, bool) {
	for _, preference := range UpdateSourcePreferences {
		if preference.Key == key {
			return preference, true
		}
	}
	return UpdateSourcePreference{}, false
}

// UpdateSourcePreferenceSubtitle explains why a source's switch is not
// available, from the update shell's latest source inventory: empty when the
// source can be toggled.
func UpdateSourcePreferenceSubtitle(states []updateflow.SourceState, ready bool, id updateflow.SourceID) string {
	if !ready {
		return "Checking availability…"
	}
	state, ok := updateSourceState(states, id)
	if !ok {
		return "Checking availability…"
	}
	if !state.Configured {
		return "Disabled by your administrator"
	}
	if !state.Available {
		return "Not available on this system"
	}
	return ""
}

// UpdateSourcePreferenceSensitive reports whether a source's switch can be
// toggled: only once the shell has checked availability, and only for a
// source the administrator enables and the host can back.
func UpdateSourcePreferenceSensitive(states []updateflow.SourceState, ready bool, id updateflow.SourceID) bool {
	if !ready {
		return false
	}
	state, ok := updateSourceState(states, id)
	return ok && state.Configured && state.Available
}

func updateSourceState(states []updateflow.SourceState, id updateflow.SourceID) (updateflow.SourceState, bool) {
	for _, state := range states {
		if state.ID == id {
			return state, true
		}
	}
	return updateflow.SourceState{}, false
}

// WelcomeViewModel models the presentation state of the first-run welcome screen.
type WelcomeViewModel struct {
	Title                  string
	Subtitle               string
	PrimaryButtonText      string
	PrimaryButtonTooltip   string
	SecondaryButtonText    string
	SecondaryButtonTooltip string
	DefaultIsPrimary       bool
	WordmarkAsset          string
}

// NewWelcomeViewModel returns a configured WelcomeViewModel for light or dark palettes.
func NewWelcomeViewModel(isDark bool) WelcomeViewModel {
	wordmark := firstrun.AssetWordmarkLight
	if isDark {
		wordmark = firstrun.AssetWordmarkDark
	}

	return WelcomeViewModel{
		Title:                  WelcomeTitle,
		Subtitle:               WelcomeSubtitle,
		PrimaryButtonText:      ConfigureEverythingAction,
		PrimaryButtonTooltip:   ConfigureEverythingDescription,
		SecondaryButtonText:    GetMovingAction,
		SecondaryButtonTooltip: GetMovingDescription(),
		DefaultIsPrimary:       true,
		WordmarkAsset:          wordmark,
	}
}
