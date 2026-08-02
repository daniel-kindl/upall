# upall

Update everything on your machine with one command.

Package managers, OS updates, and containers, behind a single tool that shows you
what it will do before it does it.

> **Status: pre-alpha. There is nothing to install yet.**
>
> The repository currently holds design documents and no application code. Work is
> tracked in [docs/ROADMAP.md](docs/ROADMAP.md); the first working binary lands at
> milestone M1. Stars and issues are welcome, downloads are not yet possible.

## What it will do

```console
$ upall
Scanning 6 providers...

  winget     12 updates   (2 need admin)
  scoop       3 updates
  docker      4 images
  windows     1 update    (needs admin, reboot)

Apply 20 updates? [y/N]
```

- **One command.** Instead of six, in whatever order you remember them.
- **Plan first.** Nothing changes until you have seen what will change and said yes.
- **Unprivileged by default.** Only the providers that genuinely need admin get it,
  and the plan tells you which ones before you answer.
- **No prerequisites.** A single downloaded binary runs. No runtime, no interpreter,
  no libraries.
- **Missing tools are fine.** Providers you do not have are skipped, not failed.
- **Honest about failure.** One provider failing does not stop the others, and the
  summary tells you exactly what broke.

A desktop client, `upall-gui`, ships alongside the CLI with the same capabilities.

## Planned providers

| Windows | Linux | Cross-platform |
|---|---|---|
| winget | apt | docker |
| scoop | dnf | podman |
| chocolatey | pacman | compose stacks |
| Windows Update | snap | |
| | flatpak | |

macOS is planned after 1.0. Language toolchain managers — npm, pipx, cargo, rustup —
are planned but deferred; see the Post-1.0 section of the roadmap.

## What it is not

- **Not a package manager.** It drives the ones you already have. It never installs
  software that was not already there.
- **Not configuration management.** No desired state, no inventory, no convergence.
- **No rollback.** The underlying tools vary too much for that to be reliable. Every
  run is journaled so you can undo things deliberately.
- **No daemon, no scheduling, no remote hosts.** systemd timers, Task Scheduler, and
  Ansible already exist and are good.

## Documentation

| | |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How it is built and the rules that hold across it |
| [docs/ROADMAP.md](docs/ROADMAP.md) | What is built, what is next, what is deliberately deferred |
| [adr/](adr/README.md) | Decisions already made, and what they cost |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Getting set up and sending a change |
| [AGENTS.md](AGENTS.md) | Commit, branching, and PR rules |
| [SECURITY.md](SECURITY.md) | Reporting a vulnerability |

There is no documentation site, deliberately. Reference documentation lives in the
code and is read with `go doc`; see
[ADR-0007](adr/0007-godoc-as-reference-documentation.md).

## Contributing

Contributions are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md), and read
[adr/](adr/README.md) before proposing anything that reverses a settled decision — the
reasoning and the trade-offs are recorded there.

Adding a provider is usually a TOML file and a test.

## License

[Apache-2.0](LICENSE).
