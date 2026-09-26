@setup
Feature: Setup assistant
  On a first run the application opens as a short wizard over the ordinary
  control panel — a welcome screen, then at most three optional steps — and
  afterwards it is the control panel. Every step's controls act through the
  page that owns the setting, every exit records a disposition, and under
  --dry-run (every scenario here) nothing is persisted or applied. The suite
  cannot read a real first run, because --dry-run suppresses the automatic
  presentation; @args.--setup opens the assistant the same way.

  Scenario: A dry run never presents the assistant uninvited
    Given ChairLift is running
    Then no dialog is shown
    And the setup dry run would record no disposition

  Scenario: The assistant opens from the main menu on its welcome screen
    Given ChairLift is running
    When I open the main menu
    And I choose "Setup Assistant…" from the menu
    Then a dialog titled "Welcome to Bluefin" is shown
    And the dialog says "Configure Everything"
    And the dialog says "Get Moving"
    And the action journal is empty

  @args.--setup
  Scenario: A launch with --setup opens the assistant over the control panel
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    And the sidebar lists every navigation page in order
    And the action journal is empty

  @args.--setup
  Scenario: Get Moving records a skip and leaves the control panel as it was
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Get Moving" in the dialog
    Then no dialog is shown
    And a toast says "You're ready to go!"
    And the setup dry run would record disposition skipped
    And the sidebar lists every navigation page in order
    And the "Updates" page is shown
    And the action journal is empty

  @args.--setup
  Scenario: Dismissing the assistant with Escape records a skip
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I press "Escape"
    Then no dialog is shown
    And the setup dry run would record disposition skipped
    And the action journal is empty

  @args.--setup @stub.livery-tools @stub.updates-flatpak-current @stub.updates-brew-current
  Scenario: Configure Everything walks the three steps and Finish records a completion
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then the setup assistant shows the "Appearance" step
    And the setup assistant offers the "Next" button
    And the setup assistant does not offer the "Finish" button
    When I choose "Next" in the dialog
    Then the setup assistant shows the "Apps" step
    When I choose "Next" in the dialog
    Then the setup assistant shows the "Update Preferences" step
    And the setup assistant offers the "Finish" button
    And the setup assistant does not offer the "Next" button
    And every switch in the setup assistant has an accessible name
    When I choose "Finish" in the dialog
    Then no dialog is shown
    And a toast says "Setup completed!"
    And the setup dry run would record disposition completed
    And the setup dry run would record the completed version
    And the action journal is empty

  @args.--setup
  Scenario: Back from the first step returns to the welcome screen
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then the setup assistant shows the "Appearance" step
    When I choose "Back" in the dialog
    Then a dialog titled "Welcome to Bluefin" is shown
    And the dialog says "Configure Everything"
    And the setup dry run would record no disposition

  @args.--setup
  Scenario: Reopening the assistant after a decision starts over at the welcome screen
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then the setup assistant shows the "Appearance" step
    When I press "Escape"
    Then no dialog is shown
    And the setup dry run would record disposition skipped
    When I open the main menu
    And I choose "Setup Assistant…" from the menu
    Then a dialog titled "Welcome to Bluefin" is shown
    And the dialog says "Configure Everything"

  @args.--setup @stub.livery-tools
  Scenario: The Appearance step drives the Livery page's own switches
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then the setup assistant shows the "Appearance" step
    And the "Customize the Panel Icon" switch in the setup assistant is off
    And the "Customize the Files Icon" switch in the setup assistant is off
    And the "Customize the Panel Icon" switch in the setup assistant is sensitive
    When I toggle the "Customize the Panel Icon" switch in the setup assistant
    Then the Livery dry run would set panel-enabled to true
    And the Livery dry run would point the panel at "chairlift-livery-cncf-symbolic"
    And the "Customize the Panel Icon" switch in the setup assistant is on
    And no Livery command changed any setting
    And no icon was written under the home directory
    And the action journal is empty
    When I press "Escape"
    And I open the "Livery" page
    Then the "Customize the Panel Icon" switch in the Livery "Foundational Livery" section is on
    And the "Customize the Files Icon" switch in the Livery "Dock Livery" section is off

  @args.--setup @stub.livery-no-extension
  Scenario: An Appearance choice this desktop cannot apply stays locked and says so
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then the setup assistant shows the "Appearance" step
    And the "Customize the Files Icon" switch in the setup assistant is sensitive
    And the "Customize the Panel Icon" row in the setup assistant says "Not available on this desktop"
    And the "Customize the Panel Icon" switch in the setup assistant is insensitive

  @args.--setup @config.apps-bundles @stub.apps-brew @stub.apps-flatpak @stub.apps-collections
  Scenario: The Apps step lists the collections the Apps page found and previews an install
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    And I choose "Next" in the dialog
    Then the setup assistant shows the "Apps" step
    And the "Coding fonts" row in the setup assistant says "Fixed-width fonts made for reading code. Includes 3 apps and tools."
    And the "Team tools" row in the setup assistant says "Tools our team relies on every day. Includes 1 app or tool."
    When I click the "Install" button in the "Coding fonts" row of the setup assistant
    Then the application log contains "[DRY-RUN] Would execute: brew bundle install --file="
    And the application log contains "/bundles/fonts-dev.Brewfile"
    And the "Install" button in the "Coding fonts" row of the setup assistant is sensitive
    And Homebrew was never asked to "bundle"
    And the action journal is empty

  @args.--setup @config.apps-bundles @stub.apps-brew @stub.apps-flatpak
  Scenario: The Apps step says when this system offers no collections
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    And I choose "Next" in the dialog
    Then the setup assistant shows the "Apps" step
    And the setup assistant says "No collections available"

  # bootc-stage is left out of the capability set, as updates.feature does,
  # so the Operating system choice is floored identically on every host; the
  # updex-backed System components source is never available on the runner.
  @args.--setup @stub.updates-flatpak-current @stub.updates-brew-current @env.CHAIRLIFT_CAPABILITIES=image-descriptor,flatpak,brew,podman
  Scenario: The Update Preferences step binds the same sources as Preferences
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    And I choose "Next" in the dialog
    And I choose "Next" in the dialog
    Then the setup assistant shows the "Update Preferences" step
    And the "Applications" switch in the setup assistant is on
    And the "Applications" switch in the setup assistant is sensitive
    And the "Developer tools" switch in the setup assistant is sensitive
    And the "System components" row in the setup assistant says "Not available on this system"
    And the "System components" switch in the setup assistant is insensitive
    When I toggle the "Applications" switch in the setup assistant
    Then the setup dry run would set the applications-enabled update preference to false
    And the "Applications" switch in the setup assistant is off
    And the action journal is empty

  @args.--setup @config.setup-no-tasks @env.CHAIRLIFT_CAPABILITIES=image-descriptor
  Scenario: With no optional task to offer, Configure Everything completes setup at once
    Given ChairLift is running
    Then a dialog titled "Welcome to Bluefin" is shown
    When I choose "Configure Everything" in the dialog
    Then no dialog is shown
    And the setup dry run would record disposition completed
    And the action journal is empty
