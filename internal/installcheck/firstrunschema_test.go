package installcheck

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/projectbluefin/chairlift/internal/firstrun"
	"github.com/projectbluefin/chairlift/internal/views/pageview"
)

// The setup assistant persists its disposition through the `gsettings` tool
// against data/io.projectbluefin.chairlift.firstrun.gschema.xml, and its
// Update Preferences step binds switches to keys of the `updates` schema by
// name. Neither binding is checked at compile time: a key renamed on one side
// makes `gsettings set` fail on every Get Moving (the assistant then returns
// on every launch), and g_settings_bind on an undeclared key aborts the
// process the moment the step is built. These gates hold each side to the
// other, headlessly, the way settingsschema_test.go holds internal/settings.

const firstrunSchemaFile = "data/io.projectbluefin.chairlift.firstrun.gschema.xml"

// TestFirstrunSchemaIDMatchesTheShippedSchema keeps firstrun.SchemaID naming
// the schema the repository installs.
func TestFirstrunSchemaIDMatchesTheShippedSchema(t *testing.T) {
	id, _ := readGSchema(t, firstrunSchemaFile)
	if id != firstrun.SchemaID {
		t.Errorf("firstrun.SchemaID = %q, %s declares id %q", firstrun.SchemaID, firstrunSchemaFile, id)
	}
}

// TestFirstrunKeysMatchTheShippedSchema holds firstrun.Keys and the schema's
// key inventory to each other, in both directions, and requires every key to
// be a string: the store quotes each value as a GVariant string.
func TestFirstrunKeysMatchTheShippedSchema(t *testing.T) {
	_, declared := readGSchema(t, firstrunSchemaFile)

	known := map[string]bool{}
	for _, key := range firstrun.Keys {
		known[key] = true
		gkey, ok := declared[key]
		if !ok {
			t.Errorf("firstrun.Keys names %q, which %s does not declare", key, firstrunSchemaFile)
			continue
		}
		if gkey.Type != "s" {
			t.Errorf("%s declares %q with type %q; the store writes it as a string", firstrunSchemaFile, key, gkey.Type)
		}
	}
	for _, key := range sortedKeys(declared) {
		if !known[key] {
			t.Errorf("%s declares key %q that firstrun.Keys does not name", firstrunSchemaFile, key)
		}
	}
}

// TestFirstrunSchemaDefaultsMatchTheDispositionModel pins the two defaults
// the code relies on: a fresh account reads as not-addressed, which is what
// ShouldPresent turns into presenting the assistant, and no version has
// completed setup.
func TestFirstrunSchemaDefaultsMatchTheDispositionModel(t *testing.T) {
	_, declared := readGSchema(t, firstrunSchemaFile)

	if got, want := strings.Trim(declared[firstrun.KeyDisposition].Default, "'\""), string(firstrun.DispositionNotAddressed); got != want {
		t.Errorf("%s default for %s = %q, want %q", firstrunSchemaFile, firstrun.KeyDisposition, got, want)
	}
	if got := strings.Trim(declared[firstrun.KeyCompletedVersion].Default, "'\""); got != "" {
		t.Errorf("%s default for %s = %q, want an empty version", firstrunSchemaFile, firstrun.KeyCompletedVersion, got)
	}
}

// TestSetupUpdateChoicesBindDeclaredBooleanKeys covers the assistant's
// Update Preferences step: every source in pageview.UpdateSourcePreferences
// is bound to its Key with g_settings_bind, which aborts on a key the
// `updates` schema does not declare, and drives a switch, so the key must be
// a boolean.
func TestSetupUpdateChoicesBindDeclaredBooleanKeys(t *testing.T) {
	_, declared := readUpdatesSchema(t)

	seen := map[string]bool{}
	for _, preference := range pageview.UpdateSourcePreferences {
		if seen[preference.Key] {
			t.Errorf("UpdateSourcePreferences lists key %q twice", preference.Key)
		}
		seen[preference.Key] = true
		key, ok := declared[preference.Key]
		if !ok {
			t.Errorf("UpdateSourcePreferences binds %q, which %s does not declare: g_settings_bind aborts", preference.Key, updatesSchemaFile)
			continue
		}
		if key.Type != "b" {
			t.Errorf("%s declares %q with type %q, but a switch is bound to it", updatesSchemaFile, preference.Key, key.Type)
		}
	}
}

// TestEveryShippedSchemaIsInstalledByMake requires `make install` to install
// every data/*.gschema.xml into the system schema directory: the Livery page
// used to be the only one asserted, and the firstrun schema is what stops
// the assistant returning on every launch.
func TestEveryShippedSchemaIsInstalledByMake(t *testing.T) {
	schemas, err := filepath.Glob(filepath.Join(RepoRoot(), "data", "*.gschema.xml"))
	if err != nil {
		t.Fatalf("glob schemas: %v", err)
	}
	if len(schemas) < 3 {
		t.Fatalf("found %d shipped schemas, want at least the livery, updates and firstrun ones", len(schemas))
	}
	sort.Strings(schemas)

	makefile := readRepoFile(t, "Makefile")
	for _, schema := range schemas {
		name := filepath.Base(schema)
		install := "install -Dm644 data/" + name + " $(DESTDIR)$(SCHEMASDIR)/" + name
		if !strings.Contains(makefile, install) {
			t.Errorf("Makefile does not install %s: want %q", name, install)
		}
		uninstall := "rm -f $(DESTDIR)$(SCHEMASDIR)/" + name
		if !strings.Contains(makefile, uninstall) {
			t.Errorf("Makefile does not uninstall %s: want %q", name, uninstall)
		}
	}
}
