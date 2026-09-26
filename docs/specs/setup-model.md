# Spec: Optional setup step model

`internal/firstrun.AssistantModel` provides the pure navigation and choice
contract consumed by the setup dialog, `internal/views.FirstRunAssistant`.
This specifies the model (issue #224) and the dialog adapter that renders
each choice as a real control (issue #225).

## Interface

| Field/API | Contract |
| --- | --- |
| `Step.ID` | Stable task identity: `theme`, `apps`, `updates`; `welcome` is the entry screen |
| `Step.Choices` | Independently filtered delivered actions/preferences |
| `Choice.ID` | Stable control identity; never interpreted as an executable command |
| `Choice.Policy` | Original `(Page, Group)` references, all required |
| `NewAssistantModel(floor)` | Snapshot the session's shared `capability.Compose` predicate |
| `Steps`, `CurrentStep`, transition results | Independent copies including nested policy references |
| `Skip(current)`, `Dismiss(current)` | Emit a disposition without persistence or optional mutations |

## Rules

- **short-flow:** There MUST be at most three optional tasks, in Appearance,
  Apps, Update Preferences order. No dedicated developer, AI or gaming tour.
  The entry screen is separate; zero optional tasks MUST exit directly on
  Configure/forward without showing an empty wizard.
- **original-policy:** Every offered choice MUST pass each original policy
  reference through the shared capability floor. Nil MUST deny every choice.
  Filtering MUST remove individual denied choices and then empty tasks.
  System component update preferences MUST use `features_page/features_group`,
  not a synthetic Updates policy. Configuration cannot elevate host support.
- **delivered-controls:** The candidate inventory references existing icon
  operations, Homebrew collections, and source preferences only. No speculative
  avatar/wallpaper backend or optional activation is required. The adapter
  renders every offered choice as a real control and MUST NOT ship a
  placeholder or a description standing in for one: an Appearance choice is
  the Livery page's own switch row, the Apps choice is one row with an
  Install button per collection the Apps page discovered, and an Update
  Preferences choice is a switch bound to the `updates` schema key spelled by
  its `Choice.ID`. The adapter MUST further restrict offers for desktop/async
  readiness (a row stays insensitive until its page has loaded, and says so)
  and it MUST NOT substitute a config-only predicate.
- **one-owner-per-setting:** The adapter MUST act through `views.SetupHost`
  — the pages' own handlers — never through a second implementation: an
  Appearance switch flips the Livery page's switch so its handler runs under
  its gate, a collection installs through the Apps page's shared
  per-collection gate so both surfaces show one phase, and an update source
  binds the same GSettings key the Preferences dialog binds. The pages are
  therefore never out of date when the dialog closes, and the main window
  and the assistant cannot start conflicting actions.
- **navigation-only:** Construction, Configure, Next and Back MUST NOT run
  tools, change preferences, install software, or alter primary navigation or
  accelerators. Moving onto the final step MUST still present it; advancing
  beyond it emits `completed`. Intermediate advances emit `not-addressed`.
- **optional-exit:** Skip and intentional Dismiss MUST be available at any
  stage and emit the same remembered effect: `skipped`, preserving an existing
  `completed` disposition on explicit reopening. A crash emits no decision.
  The adapter owns persistence (`firstrun.RecordSkip` for Get Moving and for
  a close — Escape, the close button, a click outside — that reached no
  decision), dry-run suppression and write-error reporting. Under `--dry-run`
  the adapter MUST persist nothing and bind nothing: disposition writes and
  update-preference toggles are logged as `[DRY-RUN] would set …` lines.
- **snapshot:** Returned slices MUST NOT allow callers to alter internal
  choices, policy references, or later assistant sessions.

`flow_test.go` covers zero/all/subset sequences, each original policy exclusion
and capability truth table, cross-namespace filtering, compound requirements,
Skip/dismissal/Back/Next/Finish, snapshot isolation and unchanged sidebar bindings.
`firstrun_test.go` covers the GSettings store's write shape, its dry-run
silence, its listing parser and `RecordSkip`. Tests run in the ordinary
filtered headless unit gate; they do not certify GTK interaction. The dialog
itself is certified by `test/e2e/features/setup.feature` through AT-SPI on
Dakota: presentation from the menu and from `--setup`, Get Moving, Escape,
the three steps, Back, reopening, and each step's controls acting through
its page under `--dry-run`.

## References

- Rationale: [capability floor ADR](../adr/0014-capability-driven-visibility-as-a-floor.md)
  and [pure leaf packages ADR](../adr/0007-pure-leaf-packages-route-around-untestable-gtk.md)
- Context: [architecture](../design/overview.md#optional-setup-model)
- Implementation plan: [#224](https://github.com/projectbluefin/chairlift/issues/224)
- Dialog adapter: [#225](https://github.com/projectbluefin/chairlift/issues/225),
  implemented by `internal/views/firstrun.go` over `internal/views/setup_host.go`
