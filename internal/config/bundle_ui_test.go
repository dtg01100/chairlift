package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrewBundleGroupConfigControlsRuntimeWiring(t *testing.T) {
	if !defaultConfig().IsGroupEnabled("applications_page", "brew_bundles_group") {
		t.Fatal("default brew_bundles_group is disabled, want enabled")
	}

	path := writeConfigFile(t, "applications_page:\n  brew_bundles_group:\n    enabled: false\n")
	cfg, loadErr := loadFromPath(path)
	if loadErr != nil {
		t.Fatalf("loadFromPath(%q): %v", path, loadErr)
	}
	if cfg.IsGroupEnabled("applications_page", "brew_bundles_group") {
		t.Fatal("explicitly disabled brew_bundles_group remains enabled")
	}

	// The page discovers and lists collections; the install itself lives in
	// the shared runner every surface offering a collection goes through
	// (internal/views/setup_host.go), so each file is held to its half.
	requireSource(t, filepath.Join(repoRoot(), "internal", "views", "applications_page.go"), []string{
		`if uh.groupEnabled("applications_page", "brew_bundles_group") {`,
		`uh.config.GetGroupConfig("applications_page", "brew_bundles_group")`,
		`groupCfg.BundlesPaths`,
		`go uh.loadBrewBundles(bundlePaths)`,
		`homebrew.AvailableBundles(paths)`,
		`bundleview.Present(len(bundles), warning)`,
		`bundleview.Describe(bundle.Name, bundle.Description, bundle.ItemCount)`,
		`uh.ConnectBundleInstall(bundle, installBtn)`,
	})
	requireSource(t, filepath.Join(repoRoot(), "internal", "views", "setup_host.go"), []string{
		`if !shared.gate.TryStart()`,
		`homebrew.BundleInstall(bundle.Path)`,
		`actionmsg.BundleInstall(dryrun.Enabled(), collection.Title)`,
	})
}

func requireSource(t *testing.T, sourcePath string, required []string) {
	t.Helper()
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	text := string(source)
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not contain %q", filepath.Base(sourcePath), want)
		}
	}
}
