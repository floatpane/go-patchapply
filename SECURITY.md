# Security Policy

## Supported Versions

Only the latest release of go-patchapply is supported with security updates.

## Reporting a Vulnerability

If you discover a security vulnerability in go-patchapply, please report it responsibly. **Do not open a public issue.**

Email us at [us@floatpane.com](mailto:us@floatpane.com) with:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact
- Any suggested fixes (optional)

We will acknowledge your report within 48 hours and aim to provide a fix or mitigation plan within 7 days, depending on severity.

## Scope

This policy covers the go-patchapply codebase and its official releases.

go-patchapply **writes files** from untrusted patches — a mailed patch comes from whoever sent it, and its paths and contents are attacker-controlled. The threats that matter are escaping the target directory, leaving a half-applied tree, and misapplying a hunk:

- **Path escape** — any patch path that writes outside a `DirFS` root: `../` traversal, absolute paths, or symlink tricks that `DirFS` should reject with `ErrUnsafePath`.
- **Partial application** — a code path where a failing apply leaves some files written and others not, instead of writing nothing. Application must stay all-or-nothing.
- **Silent misapplication** — a hunk placed where its context does **not** actually match, so the wrong region of a file is edited without an `ErrConflict`.
- **Resource exhaustion** — a crafted diff that drives runaway memory or super-linear CPU during hunk location.
- **Panics on malformed input** — any `FileChange` or diff that panics the applier instead of returning an error.

Note the explicit non-goals (see the docs): go-patchapply never executes git or a shell, and it does **not** vet patch *content* — it writes what the diff says, inside the root you give it. Reviewing the change and choosing a safe root are the caller's responsibility.

go-patchapply's only dependency is its sibling [go-mailpatch](https://github.com/floatpane/go-mailpatch); everything else is the Go standard library.

## Disclosure

We ask that you give us reasonable time to address the issue before disclosing it publicly. We are committed to crediting reporters in release notes (unless you prefer to remain anonymous).
