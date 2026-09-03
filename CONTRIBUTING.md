# Contributing to mini-orca

Thank you for your interest in contributing to mini-orca! We welcome contributions of all kinds, including bug fixes, new features, documentation improvements, and bug reports.

## How to Contribute

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally: `git clone https://github.com/your-username/mini-orca.git`
3.  **Create a new branch** for your changes: `git checkout -b feature/my-new-feature` or `git checkout -b fix/my-bug-fix`
4.  **Make your changes** and ensure they follow the project's code style and standards.
5.  **Write tests** for behavior changes and run the full local validation path: `make check`. Use `./desktop/gradlew -p desktop test` for desktop-only feedback; the checked-in wrapper avoids requiring a globally installed Gradle.
6.  **Commit your changes** with clear and descriptive commit messages.
7.  **Push your branch** to your fork: `git push origin feature/my-new-feature`
8.  **Submit a Pull Request** to the main repository.

## Code Style

- Follow standard Go idioms and format Go changes with `go fmt ./...`.
- Keep the preview-first boundary: source and composed diff views are read-only;
  only a reviewed declaration draft can reach the explicit, one-file Apply/Undo
  path.
- Keep daemon routes loopback-only by default and configuration credentials in
  ignored local `config.yaml`.
- Do not edit generated `build/`, `desktop/build/`, `desktop/.gradle/`, or
  `desktop/.kotlin/` output. Keep Compose changes inside `desktop/` and use its
  wrapper.
- Prefer small cohesive changes, current package boundaries, and focused tests
  over compatibility layers or speculative abstractions.

## Reporting Bugs

If you find a bug, please open an issue on GitHub with a clear description of the problem, steps to reproduce it, and any relevant logs or error messages.

## Suggesting Enhancements

We welcome suggestions for new features and enhancements! Please open an issue on GitHub to discuss your ideas.

## License

By contributing to mini-orca, you agree that your contributions will be licensed under the project's license.
