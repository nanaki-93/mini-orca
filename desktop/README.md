# Mini-Orca desktop

The desktop uses React and Wails with the Go daemon. See the
[desktop guide](webapp/README.md) for setup, usage, terminal behavior and checks.

Run `make desktop-run`, build with `make desktop-build`, and validate with
`make desktop-test` from the repository root. Native packaging supports macOS
arm64. The retired Kotlin/Compose client and its Java/Gradle toolchain are no
longer part of the repository.
