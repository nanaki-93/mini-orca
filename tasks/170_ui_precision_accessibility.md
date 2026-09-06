# Task 170 — retained working evidence

Historical status: In Progress. This note preserves the existing uncommitted
execution record. [PLAN.md](../PLAN.md) replaces the old task sequence; UI-04 owns
native/accessibility follow-up and REL-01 owns final acceptance. This file does
not authorize execution or a commit and is not a second backlog.

The component checks below passed historically. Native window, popup, OS focus
and screen-reader checks remain incomplete. The detailed operator requirements
remain in [UI_PRECISION_ACCEPTANCE.md](../desktop/UI_PRECISION_ACCEPTANCE.md)
and the canonical [release acceptance](../docs/RELEASE_ACCEPTANCE.md).
REL-02 may retire this note after consolidating its evidence and outstanding checks.

## Execution record

- Expanded the production-component matrix to render the compact `1280x600` shell at 100%, 125%, and 150% text and at 1×/2× density. The fixture asserts the named Preview, Pause, and Cancel controls remain readable at every added scale; generated evidence is ignored under `desktop/build/reports/ui-precision/task-170/`.
- Passed `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.DesktopAccessibilityTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' -PvisualOutput=/Users/marcoandreose/DEV/lab/mini-orca/desktop/build/reports/ui-precision/task-170 -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home`; inspected the 150% and 2× compact captures.
- Passed `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop spotlessCheck detekt test -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home` and `git diff --check`. JBR emitted its known restricted-native-access and Jewel `Unsafe` deprecation warnings; no runtime or configuration change was made.
- Launched the actual desktop app with JBR 25.0.4 on macOS 26.6.2 and observed the live `io.miniorca.desktop.MainKt` process. The available computer-use inventory exposed no native applications while it ran, preventing native screenshots, window-edge popup/dialog inspection, OS focus traversal, resizing, or an accessibility-tree read; the temporary process was stopped.
- No supported screen reader was running or exposed to this environment. Native and assistive-technology evidence is therefore incomplete. Task 170 remains In Progress, no passing acceptance commit has been created, and the required operator matrix is recorded in `desktop/UI_PRECISION_ACCEPTANCE.md`.
