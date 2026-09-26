package pageview

import (
	"strconv"
	"strings"
	"testing"

	"github.com/projectbluefin/chairlift/internal/printerapp"
)

func TestPrintersGroupDescriptionSaysWhereItRuns(t *testing.T) {
	d := PrintersGroupDescription()
	for _, want := range []string{"your account", "adds nothing to the system", "network"} {
		if !strings.Contains(d, want) {
			t.Errorf("description %q does not say %q", d, want)
		}
	}
}

// Every family gets a distinct title that names the family, so a person can
// match the row to the driver a printer's documentation names.
func TestPrinterFamilyRowNamesEveryFamilyDistinctly(t *testing.T) {
	seen := map[string]string{}
	for _, f := range printerapp.Families() {
		title := PrinterFamilyRow(f)
		if !strings.Contains(title, f.DisplayName) {
			t.Errorf("%s row title %q does not name the family", f.ID, title)
		}
		if prev, dup := seen[title]; dup {
			t.Errorf("families %s and %s share title %q", prev, f.ID, title)
		}
		seen[title] = f.ID
	}
	if got := PrinterFamilyRow(printerapp.Family{ID: "hplip", DisplayName: "HPLIP"}); !strings.HasPrefix(got, "HP printers") {
		t.Errorf("HPLIP row = %q; a person looks for HP, not HPLIP", got)
	}
}

// Every state reads differently, and only Ready says running: a starting,
// failed, or blocked family can never be mistaken for one that works.
func TestPrinterAppSubtitleDistinguishesEveryState(t *testing.T) {
	seen := map[string]printerapp.State{}
	for s := printerapp.StateUnavailable; s <= printerapp.StateFailed; s++ {
		text := PrinterAppSubtitle(s, 18010)
		if prev, dup := seen[text]; dup {
			t.Errorf("states %d and %d share subtitle %q", prev, s, text)
		}
		seen[text] = s
		if s != printerapp.StateReady && strings.HasPrefix(text, "Running") {
			t.Errorf("state %d claims running: %q", s, text)
		}
	}
}

// ADR-0016: the blocked row is an actionable, non-enabled state. It says the
// administration page is what cannot be secured, what the image must accept,
// and that the switch unlocks — without spelling an environment variable no
// image ships yet.
func TestPrinterAppBlockedSubtitleIsActionable(t *testing.T) {
	text := PrinterAppSubtitle(printerapp.StateBlocked, 18010)
	for _, want := range []string{"Can't be turned on yet", "administration page", "secured", "credential", "unlocks"} {
		if !strings.Contains(text, want) {
			t.Errorf("blocked subtitle %q does not say %q", text, want)
		}
	}
	if strings.Contains(text, "PRINTER_APP") || strings.Contains(text, "_") {
		t.Errorf("blocked subtitle spells an unshipped knob: %q", text)
	}
}

// The ready row is where a person goes next: the web page on the app's port.
func TestPrinterAppReadySubtitleNamesTheWebPage(t *testing.T) {
	for _, f := range printerapp.Families() {
		app := printerapp.Select(f)
		text := PrinterAppSubtitle(printerapp.StateReady, app.Port())
		want := "http://localhost:" + strconv.Itoa(app.Port()) + "/"
		if !strings.Contains(text, want) {
			t.Errorf("%s ready subtitle %q does not name %q", f.ID, text, want)
		}
	}
}

// A failed stop keeps the unit because the service may still run, so the
// toast has to keep saying it is running, and name which row it is about.
func TestPrinterAppFailureToastNamesTheRowAndDoesNotClaimAStop(t *testing.T) {
	off := strings.ToLower(PrinterAppFailureToast(false, "HP printers (HPLIP)"))
	if !strings.Contains(off, "hp printers") || !strings.Contains(off, "still running") || !strings.Contains(off, "nothing was removed") {
		t.Errorf("failed-disable toast = %q", off)
	}
	on := PrinterAppFailureToast(true, "Gutenprint printers")
	if !strings.HasPrefix(on, "Gutenprint printers") || !strings.Contains(on, "could not be turned on") {
		t.Errorf("failed-enable toast = %q", on)
	}
}
