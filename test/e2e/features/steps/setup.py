"""Steps for the setup assistant (features/setup.feature).

The assistant is an in-window AdwDialog: a hero screen, then one step at a
time, each an AdwPreferencesGroup titled by the step. Every lookup here is
scoped to the showing dialog, so a row the Livery page also carries (the
Appearance step reuses the Livery page's own switch rows) cannot satisfy an
assertion about the assistant.
"""

from behave import step, then

import chairlift_atspi as atspi

FIRSTRUN_SCHEMA = "io.projectbluefin.chairlift.firstrun"
UPDATES_SCHEMA = "io.projectbluefin.chairlift.updates"


# ---------------------------------------------------------------- lookups


# behave executes every steps module itself, so importing steps/common.py
# would register its steps a second time; these lookups are repeated.
def _app(context):
    if context.app is None:
        raise AssertionError("ChairLift is not running in this scenario (@no-app?)")
    return context.app


def read_log(context):
    try:
        with open(context.log_path, "r", encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except (AttributeError, OSError):
        return ""


def text_present(root, wanted, exact=False):
    return any((value == wanted) if exact else (wanted in value) for value in atspi.all_text_under(root))


def current_dialog(context, timeout=atspi.DEFAULT_TIMEOUT):
    """The most recently presented in-window dialog (toasts are alerts, not
    dialogs, and are skipped)."""
    def lookup():
        found = None
        for node in atspi.descendants(_app(context), only_showing=True):
            if atspi.role(node) in ("dialog", "alert") and not atspi.is_toast(node):
                found = node
        return found

    dialog = atspi.poll(lookup, timeout=timeout)
    if dialog is None:
        raise AssertionError(f"no dialog is showing after {timeout}s")
    return dialog


def _assistant(context, timeout=atspi.DEFAULT_TIMEOUT):
    return current_dialog(context, timeout=timeout)


def _step_group(context, title):
    """The showing step's preferences group, published as a grouping."""
    return atspi.find(
        _assistant(context),
        lambda n: atspi.role(n) == "grouping" and atspi.name(n) == title,
        f"a setup step titled {title!r}",
    )


def _row(context, title):
    return atspi.row_containing(_assistant(context), title)


def _switch(context, row):
    return atspi.find(
        _row(context, row),
        lambda n: atspi.role(n) == "switch",
        f"a switch in the {row!r} row of the setup assistant",
    )


def _texts(root):
    return [v for n in atspi.descendants(root, only_showing=True) for v in (atspi.name(n), atspi.text(n)) if v]


# ---------------------------------------------------------------- steps and buttons


@then('the setup assistant shows the "{title}" step')
def step_shows_step(context, title):
    """The step's group is showing and the dialog is titled for it."""
    _step_group(context, title)
    ok = atspi.poll(lambda: text_present(_assistant(context), title, exact=True))
    assert ok, f"the setup assistant never titled itself {title!r}"


@then('the setup assistant offers the "{label}" button')
def step_offers_button(context, label):
    button = atspi.find_button(_assistant(context), label)
    assert atspi.sensitive(button), f"the {label!r} button is insensitive"


@then('the setup assistant does not offer the "{label}" button')
def step_lacks_button(context, label):
    gone = atspi.poll(
        lambda: not atspi.find_all(_assistant(context), lambda n: atspi.is_button(n, label))
    )
    assert gone, f"the setup assistant still offers a {label!r} button"


@step('I click the "{label}" button in the "{row}" row of the setup assistant')
def step_click_row_button(context, label, row):
    atspi.activate(atspi.find_button(_row(context, row), label))


@then('the "{label}" button in the "{row}" row of the setup assistant is {state:w}')
def step_row_button_state(context, label, row, state):
    if state not in ("sensitive", "insensitive"):
        raise NotImplementedError(f"unknown button state {state!r}")
    want = state == "sensitive"

    def check():
        return atspi.sensitive(atspi.find_button(_row(context, row), label, timeout=1)) == want

    assert atspi.poll(check), f"the {label!r} button in the {row!r} row never became {state}"


# ---------------------------------------------------------------- switches and rows


@step('I toggle the "{row}" switch in the setup assistant')
def step_toggle(context, row):
    switch = _switch(context, row)
    assert atspi.poll(lambda: atspi.sensitive(switch)), f"the {row!r} switch in the setup assistant is insensitive"
    atspi.activate(switch)


@then('the "{row}" switch in the setup assistant is {state:w}')
def step_switch_state(context, row, state):
    checks = {
        "on": lambda s: bool(atspi.checked(s)),
        "off": lambda s: not atspi.checked(s),
        "sensitive": atspi.sensitive,
        "insensitive": lambda s: not atspi.sensitive(s),
    }
    if state not in checks:
        raise NotImplementedError(f"unknown switch state {state!r}")
    ok = atspi.poll(lambda: checks[state](_switch(context, row)))
    assert ok, f"the {row!r} switch in the setup assistant never became {state}"


@then('the "{row}" row in the setup assistant says "{text}"')
def step_row_says(context, row, text):
    ok = atspi.poll(lambda: text in _texts(_row(context, row)))
    assert ok, f"the {row!r} row never said {text!r}; it says {_texts(_row(context, row))}"


@then('the setup assistant says "{text}"')
def step_assistant_says(context, text):
    ok = atspi.poll(lambda: text_present(_assistant(context), text))
    assert ok, f"the setup assistant never said {text!r}"


@then('every switch in the setup assistant has an accessible name')
def step_switches_named(context):
    switches = atspi.find_all(_assistant(context), lambda n: atspi.role(n) == "switch")
    assert switches, "the setup assistant shows no switches"
    nameless = [atspi.role(s) for s in switches if not atspi.label_text(s)]
    assert not nameless, f"{len(nameless)} switches have no accessible name"


# ---------------------------------------------------------------- persistence


def _would_set(schema, key, value):
    return f"[DRY-RUN] would set {schema} {key}={value}"


@then("the setup dry run would record disposition {value:w}")
def step_would_record(context, value):
    line = _would_set(FIRSTRUN_SCHEMA, "disposition", value)
    ok = atspi.poll(lambda: line in read_log(context))
    assert ok, f"chairlift.log never contained {line!r}"


@then("the setup dry run would record no disposition")
def step_would_record_nothing(context):
    prefix = _would_set(FIRSTRUN_SCHEMA, "disposition", "")
    assert prefix not in read_log(context), f"chairlift.log unexpectedly contains {prefix!r}"


@then("the setup dry run would record the completed version")
def step_would_record_version(context):
    prefix = _would_set(FIRSTRUN_SCHEMA, "completed-version", "")
    ok = atspi.poll(lambda: prefix in read_log(context))
    assert ok, f"chairlift.log never contained {prefix!r}"


@then("the setup dry run would set the {key:S} update preference to {value:w}")
def step_would_set_preference(context, key, value):
    line = _would_set(UPDATES_SCHEMA, key, value)
    ok = atspi.poll(lambda: line in read_log(context))
    assert ok, f"chairlift.log never contained {line!r}"
