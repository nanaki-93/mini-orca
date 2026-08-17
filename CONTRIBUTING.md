# Contributing to mini-orca

Thank you for your interest in contributing to mini-orca! We welcome contributions of all kinds, including bug fixes, new features, documentation improvements, and bug reports.

## How to Contribute

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally: `git clone https://github.com/your-username/mini-orca.git`
3.  **Create a new branch** for your changes: `git checkout -b feature/my-new-feature` or `git checkout -b fix/my-bug-fix`
4.  **Make your changes** and ensure they follow the project's code style and standards.
5.  **Write tests** for your changes and run the full local validation path: `make check`. Use `./desktop/gradlew -p desktop test` for desktop-only feedback; the wrapper avoids requiring a globally installed Gradle.
6.  **Commit your changes** with clear and descriptive commit messages.
7.  **Push your branch** to your fork: `git push origin feature/my-new-feature`
8.  **Submit a Pull Request** to the main repository.

## Code Style

- Follow standard Go idioms and best practices.
- Use `go fmt` to format your code.
- Ensure your code is well-documented with comments where necessary.
- Follow SOLID principles and avoid code duplication.

## Reporting Bugs

If you find a bug, please open an issue on GitHub with a clear description of the problem, steps to reproduce it, and any relevant logs or error messages.

## Suggesting Enhancements

We welcome suggestions for new features and enhancements! Please open an issue on GitHub to discuss your ideas.

## License

By contributing to mini-orca, you agree that your contributions will be licensed under the project's license.
