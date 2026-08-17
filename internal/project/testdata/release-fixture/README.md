# Mini-Orca release fixture

This intentionally small project exercises source indexing, selected-symbol
generation, ignored secrets, a test file, and a malformed source diagnostic.
The malformed source uses a fixture extension so language tooling never tries
to compile it.
The `.env`, certificate-shaped file, and configuration file contain placeholders
only; they exist to prove the context policy excludes them. Tests copy this
fixture to a temporary directory before importing it. Git-specific acceptance
checks initialize one copied variant as a repository and leave another as a
non-Git project.
