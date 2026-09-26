package pageview

import (
	"fmt"

	"github.com/projectbluefin/chairlift/internal/printerapp"
)

// PrintersGroupTitle is the Features page's printer applications heading.
func PrintersGroupTitle() string {
	return "Printers"
}

// PrintersGroupDescription says what turning a family on does and where it
// runs: in the user's own account, as a container, with nothing added to
// the system — the posture that keeps it off the pkexec path.
func PrintersGroupDescription() string {
	return "Driver services for printers that need more than built-in driverless printing. Each runs as a container in your account, adds nothing to the system, and shares its printers with this computer and your network."
}

// familyRowTitles overrides the derived "<family> printers" title where the
// family name alone would not tell a person which printers it is for.
var familyRowTitles = map[string]string{
	"hplip": "HP printers (HPLIP)",
}

// PrinterFamilyRow is the switch row's title for one driver family.
func PrinterFamilyRow(f printerapp.Family) string {
	if title, ok := familyRowTitles[f.ID]; ok {
		return title
	}
	return f.DisplayName + " printers"
}

// PrinterAppSubtitle is the switch row's subtitle for a resolved state. The
// blocked text is the ADR-0016 condition as a person meets it: it names what
// cannot be secured, what is needed, and that the switch unlocks once the
// image accepts it — an actionable, non-enabled state, never a switch that
// silently does nothing. port is where the ready application serves both
// IPP and its web page.
func PrinterAppSubtitle(s printerapp.State, port int) string {
	switch s {
	case printerapp.StateUnavailable:
		return "Not available on this computer — Podman is not installed."
	case printerapp.StateBlocked:
		return "Can't be turned on yet. This printer application's administration page cannot be secured until its image accepts an administrator credential; the switch unlocks once it does."
	case printerapp.StateStarting:
		return "Starting… The driver image is downloaded the first time, which can take a few minutes."
	case printerapp.StateReady:
		return fmt.Sprintf("Running. Add and manage its printers at http://localhost:%d/", port)
	case printerapp.StateFailed:
		return "Turned on, but the printer application is not running. Turn it off and on again to restart it."
	default:
		return fmt.Sprintf("Turning this on starts the driver service in your account and shares its printers on port %d.", port)
	}
}

// PrinterAppWorkingSubtitle is shown while the switch is acting.
func PrinterAppWorkingSubtitle(enabling bool) string {
	if enabling {
		return "Turning on…"
	}
	return "Turning off…"
}

// PrinterAppFailureToast is the toast for a switch that could not do what
// was asked, naming the row so three families' toasts are distinguishable.
// The error itself is logged: it names commands and unit files. A failed
// disable keeps the unit, because the service could not be proven stopped.
func PrinterAppFailureToast(enabling bool, rowTitle string) string {
	if enabling {
		return rowTitle + " could not be turned on."
	}
	return rowTitle + " is still running. It could not be stopped, so nothing was removed."
}
