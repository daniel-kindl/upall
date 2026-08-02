<!--
PRs go to `dev`, never to `main`. A PR targeting `main` will be closed rather than
retargeted — see AGENTS.md#branching.
-->

## What this changes

<!-- One or two sentences. If you need the word "and", this is probably two PRs. -->

## Milestone criterion

<!--
Which ROADMAP acceptance criterion does this advance? Quote it, and tick its box in
docs/ROADMAP.md in this PR.

If it advances none — a bug fix, a dependency bump — say so and delete the rest.
-->

- Milestone:
- Criterion:

## Why

<!--
The reasoning, if it is not obvious from the change. Especially if you chose an
approach that looks odd, or rejected one that looks obvious.

Reversing a decision recorded in adr/ needs a superseding ADR in this PR, not a
paragraph here.
-->

## Checklist

- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/), and
      a breaking change carries `!` or a `BREAKING CHANGE:` footer.
- [ ] One thing per PR. Refactors are not mixed with behavior changes.
- [ ] Tests are in this PR, not a follow-up.
- [ ] No test invokes a real package manager — `internal/exec` is faked.
- [ ] Every exported identifier added has a doc comment that says something useful.
- [ ] Nothing describing one package's internals was added to `docs/` — that goes in
      its `doc.go`.
- [ ] Any subprocess is invoked with an argv array, never a shell string.
- [ ] Platform-specific code is behind build tags, not `runtime.GOOS` branches.
- [ ] Nothing below `internal/cli` prints, prompts, or checks for a TTY.
- [ ] CI is green on `windows-latest` and `ubuntu-latest`.
- [ ] ROADMAP boxes completed by this PR are ticked.

<!--
Could not test on one platform? Say which and why. CI covers it, and saying so is
better than leaving a reviewer to guess.
-->
