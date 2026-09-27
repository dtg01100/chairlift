package ublue

import (
	"encoding/xml"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectbluefin/chairlift/internal/dryrun"
	"github.com/projectbluefin/chairlift/internal/ubluehelper"
)

// The GUI and the operating system image ship on different schedules: the
// GUI through the Homebrew cask, the helper and its PolicyKit policy through
// the image, because a cask cannot install root-owned files. An image can
// therefore be older than the GUI running on it, or carry no helper at all.
// ChairLift decides which privileged actions to offer from what the image
// actually authorizes, and hides the rest rather than letting a person
// authenticate for an action that cannot run.
//
// The authority is the installed policy, not the helper binary: pkexec runs
// the helper for an action only when an installed action names HelperPath as
// its exec.path and the command as its exec.argv1. Reading those files is a
// plain file read, so the answer needs no privilege and no subprocess, and it
// is correct for every image ever shipped, including ones whose helper
// predates this check.

// policyActionsDir is where PolicyKit reads actions. It is a variable so
// tests can point it at a fixture directory; production never writes it.
var policyActionsDir = "/usr/share/polkit-1/actions"

// statHelper reports whether the helper binary is installed. It is a seam
// for the same reason policyActionsDir is.
var statHelper = os.Stat

// policyConfig is the part of a PolicyKit .policy file this check reads.
type policyConfig struct {
	Actions []struct {
		Annotations []struct {
			Key   string `xml:"key,attr"`
			Value string `xml:",chardata"`
		} `xml:"annotate"`
	} `xml:"action"`
}

// authorizedCommands returns every helper command this host can run: the
// helper is installed at HelperPath, and an installed PolicyKit action
// authorizes the command against HelperPath. It never returns a command the
// helper does not accept, so a stray action cannot enable a control. A host
// that authorizes nothing gets nil.
func authorizedCommands() map[string]bool {
	info, err := statHelper(HelperPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}

	var commands map[string]bool
	accepted := make(map[string]bool)
	for _, command := range ubluehelper.SupportedCommands() {
		accepted[command] = true
	}

	entries, err := os.ReadDir(policyActionsDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("helper policy scan: reading %s: %v", policyActionsDir, err)
		}
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".policy") {
			continue
		}
		path := filepath.Join(policyActionsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("helper policy scan: reading %s: %v", path, err)
			continue
		}
		var config policyConfig
		if err := xml.Unmarshal(data, &config); err != nil {
			log.Printf("helper policy scan: parsing %s: %v", path, err)
			continue
		}
		for _, action := range config.Actions {
			var execPath, argv1 string
			for _, annotation := range action.Annotations {
				switch annotation.Key {
				case "org.freedesktop.policykit.exec.path":
					execPath = strings.TrimSpace(annotation.Value)
				case "org.freedesktop.policykit.exec.argv1":
					argv1 = strings.TrimSpace(annotation.Value)
				}
			}
			if execPath == HelperPath && accepted[argv1] {
				if commands == nil {
					commands = make(map[string]bool)
				}
				commands[argv1] = true
			}
		}
	}
	return commands
}

// detectCommands is the command set Detect records. A dry-run session never
// invokes the helper, so it offers every command the GUI knows: that is what
// lets a dry run preview the privileged surface on a host whose image has not
// shipped it yet, and what the screenshot walkthrough and the AT-SPI suite
// capture.
func detectCommands() map[string]bool {
	if dryrun.Enabled() {
		all := make(map[string]bool)
		for _, command := range ubluehelper.SupportedCommands() {
			all[command] = true
		}
		return all
	}
	return authorizedCommands()
}

// Supports reports whether this host can run every one of the given helper
// commands. The views hide a control that needs a command this image does not
// provide.
func (s Status) Supports(commands ...string) bool {
	if len(commands) == 0 {
		return false
	}
	for _, command := range commands {
		if !s.Commands[command] {
			return false
		}
	}
	return true
}
