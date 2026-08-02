# 0003. Per-provider elevation

**Status:** Accepted
**Date:** 2026-08-02

## Context

Updating system packages needs administrator or root. Updating a user-scoped tool does
not. In a typical run both kinds are present: `apt` needs root, `flatpak --user` does
not; Windows Update needs Administrator, `scoop` deliberately does not.

The people running upall are not thinking about privilege while they run it. They
typed one command to update their machine. Whatever upall does by default becomes the
privilege posture for every user of the tool, and it is not a decision they will
revisit.

There is also an ordering problem. The plan phase is read-only and never needs
privilege, but if elevation is acquired at startup it runs elevated anyway.

## Decision

upall runs unprivileged. Providers declare whether **applying** requires elevation;
planning never does. At apply time, only the providers that declared the need are
elevated, using `sudo` on Linux and a UAC re-exec of a helper on Windows. Everything
else runs as the invoking user.

The rendered plan marks which entries will require elevation, before the confirmation
prompt.

If elevation is unavailable or refused, those providers are reported as blocked, with
the command to run manually. The rest of the run proceeds.

## Consequences

- The read-only phases of every run (discover, detect, plan) never run elevated. This
  is most of what upall does, and all of what `upall plan` does.
- A provider that never declared elevation cannot acquire it. Bugs and bad manifests
  are contained to the privilege level they asked for.
- The user sees which parts need admin before answering the prompt, rather than
  discovering it via a UAC dialog mid-run.
- Refusing elevation degrades the run instead of failing it. You get everything that
  did not need admin, plus a list of what you would have to do yourself.
- **Two privilege contexts in one process tree**, which is the real cost: a Windows
  helper binary to build, sign, and ship, and a more complicated apply phase.
- **More prompts.** Elevating per provider can mean several sudo prompts in a run
  where elevating once would mean one. Mitigations exist, since sudo's timestamp cache
  covers the common case, but on Windows each UAC elevation is a dialog and there is
  no equivalent.
- Providers needing elevation only sometimes must declare it always, and are
  over-privileged in the cases where they did not need it. The declaration has no
  conditional form, deliberately. A conditional would be evaluated by the provider
  itself, which is the thing being constrained.

## Alternatives considered

### Re-exec the whole run elevated

Detect at startup that something will need privileges, re-exec the entire process
under sudo or UAC, and proceed. One privilege boundary, one prompt, no helper process,
considerably less code. This is what most tools in this genre do.

Rejected because it runs every provider as root, including the ones that explicitly
went out of their way not to need it. `scoop` and `flatpak --user` install into the
user's home directory by design. Running them as root creates root-owned files in a
user's home and breaks the next unprivileged run. It also elevates the plan phase,
which never needs it.

The convenience argument is real, and this is the closest call of the seven ADRs. It
was decided on the principle that a tool which quietly runs everything as root teaches
its users a bad default they will carry elsewhere.

### Never elevate

Do only what the current user can do, and report everything else as blocked with the
command to run manually. Trivially safe, no helper binary, no UAC handling at all.

Rejected because it fails at the actual job. On a typical Linux machine the OS packages
are the bulk of what needs updating, and a tool that updates everything except the
important part is a tool nobody keeps using. It survives as the fallback behavior when
elevation is refused, which is the right place for it.
