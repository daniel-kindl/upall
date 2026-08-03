# Product brief

What upall is, who it is for, how it should feel, and what the medium it renders into
will and will not allow.

## Who this is for

A design team producing a design system for upall, reading this cold with no prior
exposure to the project. It should be enough to start from without opening anything
else.

It is not a specification. [ARCHITECTURE.md](ARCHITECTURE.md) is the authority on how
the system is built, [ROADMAP.md](ROADMAP.md) on what exists and when the rest arrives,
and [the ADRs](../adr/README.md) on decisions already settled. Where this document
summarises any of them, the linked file wins.

That sourcing rule matters here more than it usually would. The project's standing rule
is that a fact written in two places eventually disagrees with itself, and nothing
breaks when it does, so nobody notices. This document deliberately breaks that rule in
order to be self-contained, which is a debt rather than a feature. It links rather than
restates wherever it can, keeps its summaries short, and should be re-read against its
sources rather than trusted after they change.

---

## The product

upall updates everything on a machine — OS package managers, OS updates, and containers
— behind one command, on Windows and Linux, as self-contained binaries that need
nothing installed to run.

It ships as two binaries over one shared core. `upall` is the command-line tool and the
primary interface. `upall-gui` is a desktop client with the same capabilities, built on
the same core, with no logic of its own.

The everyday flow is three steps, and the middle one is the product:

1. **Plan.** Ask every installed tool what it would update. Read-only. Nothing changes.
2. **Confirm.** Show the plan. Wait for a yes.
3. **Apply.** Do it, and report honestly what happened.

### What it promises

- **One command updates the machine.** The point is to stop remembering six commands.
- **Nothing is installed to use it.** A downloaded binary runs. No runtime, no
  interpreter, no shared libraries.
- **Nothing surprising happens.** You see the plan before anything changes.
- **A missing tool is not a failure.** Machines have different software on them.
- **Honest reporting.** Partial success is reported as partial success.

### What it refuses to be

These are settled. Reopening one requires a decision record, not an opinion, so a design
that assumes any of them will not be built.

**Not a package manager.** It never installs software that was not already there. It
drives the tools that do.

**Not configuration management.** No desired state, no convergence, no inventory.

**No rollback.** The underlying tools disagree too much about downgrading for a rollback
to be reliable in exactly the situation you would reach for it. Every run is journaled
so you can undo things deliberately.

**No daemon and no scheduling.** systemd timers and Task Scheduler already exist.

**No remote or multi-host operation.** upall updates the machine it runs on. There is no
fleet, no dashboard of machines, and no agent.

### Where the project actually is

Milestone 1 of 14. Both binaries build and `upall version` prints. Nothing updates
anything yet. The GUI is an empty 900×640 window, and the code comment in it is the only
layout decision anyone has made.

The GUI lands at **M11 and M12** — nine milestones out. That timeline is why the ask
below is split into work that will still be correct then, and work that will not be.

---

## Two audiences, one design language

The two frontends are aimed at different people, and this is deliberate.

**The CLI is for power users, developers, servers, and CI.** Someone who lives in a
terminal, already knows what apt and winget are, wants density over hand-holding, and
will pipe the output into something. It is the interface that works over SSH, and the
one that is measured for scriptability.

**The GUI is for regular desktop users.** Someone who wants their machine updated and
would rather click than remember a command. They may not know what a package manager is,
and should not need to.

> **The rule: one design language, two registers.**
>
> Same colours meaning the same things. Same state vocabulary. Same iconography intent.
> Same words for the same concepts. What differs between them is **density and how much
> is explained** — never what something is called, and never what it means.

Two consequences worth stating plainly, because both are easy to get wrong:

**The GUI is not a simplified subset.** ROADMAP M12 requires that anything the CLI can
do, the GUI can do, and that any difference is listed as deliberate. Detail may be
folded away. It may not be absent.

**The CLI is not exempt from design.** Its plan rendering is the product's core artifact
and it ships years before the GUI does. If it is not designed, it will be improvised by
whoever writes M5.

### One terminology decision, left open on purpose

The noun **provider** — one thing that can update software: winget, apt, docker, Windows
Update — is not just internal vocabulary. It is a config key, the argument to `--only`
and `--except`, a field in the JSON output schema, and the language of the
[provider request template](../.github/ISSUE_TEMPLATE/provider_request.yml). All of
those are under semver.

So the GUI can explain what a provider is, illustrate it, or lead with the concrete tool
names instead. Renaming it in the interface is also available, but it splits the
vocabulary between what a user sees and what the documentation, config file, and issue
tracker say. That is a real cost and possibly worth paying. Make the call deliberately
rather than by default.

---

## Personality

The project's existing documents are the tonal reference — [ADR-0005](../adr/0005-fyne-for-the-gui-client.md)
and [CONTRIBUTING.md](../CONTRIBUTING.md) especially. They are direct, unglamorous, and
allergic to marketing language. Every claim is paired with its cost. Nothing is
oversold.

The interface should read the same way.

- **Calm.** This is a tool that runs package managers as root. It should feel like it is
  taking that seriously.
- **Honest before reassuring.** Partial success is reported as partial success, not
  rounded up to a tick.
- **Never celebratory.** No "All done! 🎉". A finished run states what it did.
- **Legible rather than decorated.** Confidence comes from the user being able to read
  what happened, not from polish applied on top of it.
- **Quiet when idle.** Most of a run is waiting on somebody else's downloader. That is
  not an opportunity for entertainment.

Concretely, that rules out progress theatre, celebration states, animated flourishes on
completion, and any copy that congratulates the user for pressing a button.

---

## The interaction, in detail

This section is the part a design system has to serve. Everything here is already
specified behaviour, not an invitation.

### Providers and their states

A run discovers providers, asks which are present, asks each present one what it would
update, aggregates that into a plan, shows it, confirms, applies, and reports.

Every provider passes through these states, and each needs a visual and verbal treatment
in both media:

| State | Meaning |
|---|---|
| **Absent** | Not installed on this machine. The normal case, not a degraded one. |
| **Detected, nothing to do** | Present, checked, already current. |
| **Has updates** | Present, with a count and a list. |
| **Needs elevation** | Will require admin or root to apply. Known **before** the prompt. |
| **In progress** | Currently applying. |
| **Succeeded** | Applied cleanly. |
| **Failed** | Ran and failed. Carries a captured stderr tail. |
| **Blocked** | Elevation refused or unavailable. Not attempted. Carries the manual command. |
| **Timed out** | Exceeded its deadline. Carries the deadline. |
| **Cancelled** | Ctrl-C or the GUI's cancel arrived mid-run. |

### The rendering contract

[ARCHITECTURE.md](ARCHITECTURE.md#error-taxonomy) already fixes how the failure states
present. These are requirements, not defaults:

- **Absent is silent unless asked.** Most machines will not have most providers. Listing
  eight things you do not have, every run, above the four you do, is the failure mode to
  design against.
- **Failed is named, with its captured stderr tail.** Truncated when rendered; the
  journal keeps more.
- **Blocked shows the exact command to run manually.** The user is being handed a way
  out, and it has to be copyable.
- **Timed out names its deadline.**
- **Cancelled distinguishes what finished from what did not.**

### Three things that are load-bearing

**Elevation is marked in the plan, before the prompt.** upall runs unprivileged and
elevates only the providers that declared they need it. The user is told which ones
those are *while deciding*, not afterwards. This is a safety property expressed as a
display requirement — see [ADR-0003](../adr/0003-per-provider-elevation.md).

**Partial success is the normal case.** One provider failing does not stop the others.
A run where three succeeded, one failed, and one was blocked is ordinary, not
exceptional. Design that state first; the all-green run is the easy one.

**A reboot may be required and upall will never perform it.** Windows Update reports
pending reboots and the summary has to surface that without the interface appearing to
offer the reboot.

### The confirmation

Bare `upall` shows the plan and prompts once. Any answer but yes exits cleanly having
changed nothing, and that is a success, not a cancellation.

In the CLI this is a terminal prompt. In the GUI it is a dialog. Both satisfy the same
interface in the core, so they are two renderings of one moment and should read as such.

`--yes` skips it for CI. If input is not a terminal and `--yes` was not passed, upall
refuses rather than silently applying.

### Why the CLI cannot be chatty

Exit codes are a public contract: `0` success, `1` a provider failed, `2` usage error or
refusal, `130` interrupted. `--json` emits a versioned schema and **nothing else on
stdout**. Output is going to be parsed, piped, and logged, so decoration has to be
separable from content rather than baked into it.

---

## The medium

This is the section that decides whether the design system can be built.

### What was chosen, and what it costs

The GUI is [Fyne](https://fyne.io), decided in
[ADR-0005](../adr/0005-fyne-for-the-gui-client.md). Fyne is pure Go and draws its own
widgets through OpenGL rather than borrowing a system webview. That is what keeps the
no-prerequisites promise on Linux, where a webview toolkit would require WebKitGTK to be
installed — a package-manager prerequisite for the GUI of a tool whose purpose is
managing package managers.

The ADR states the cost in its own words:

> **The UI will not look as good.** Fyne draws its own widgets in its own idiom, so it
> looks like Fyne rather than like Windows or GNOME. Layout and styling are a Go widget
> tree, which is more laborious and less expressive than CSS for anything visually
> ambitious. This is a real, permanent, user-visible cost.

That decision is accepted and is not what this engagement is revisiting. The webview
alternative was considered at length and rejected as the most expensive rejection in the
project's decision records. A design that concludes "you should have used HTML" is
answering a question that is closed.

**One correction to the ADR's framing, which is useful to have.** ADR-0005 was written
against an older version. The pinned version is **Fyne v2.8.0**, and 2.8 raised the
ceiling noticeably: real shadows, a blur primitive, per-subtree theme overrides, and
shader animation all arrived in it. What follows was checked against the pinned module
rather than inherited from the ADR's summary, and is more capable than that summary
suggests.

### What you can control

Fyne's theme is a closed, enumerable set of tokens, and — this is the useful part — the
whole thing loads from **a single JSON file** at runtime via `theme.FromJSON`. So the
design system has a literal delivery format that can be checked mechanically.

The JSON schema is four maps:

```json
{
  "Colors":       { },
  "Colors-light": { },
  "Colors-dark":  { },
  "Sizes":        { },
  "Fonts":        { },
  "Icons":        { }
}
```

**30 colour tokens**, with `Colors-light` and `Colors-dark` overriding a shared `Colors`
base independently — so light and dark are genuinely separate designs, not one derived
from the other:

```
background  foreground  primary  focus  hover  pressed  selection  disabled
button  disabledButton  inputBackground  inputBorder  placeholder
menuBackground  headerBackground  overlayBackground  hyperlink
separator  shadow  scrollBar  scrollBarBackground
error  warning  success
foregroundOnPrimary  foregroundOnError  foregroundOnWarning  foregroundOnSuccess
innerWindowBorder  innerWindowBorderInactive
```

**27 size tokens**, which is where the spacing and shape system lives:

```
text  headingText  subHeadingText  helperText  lineSpacing
padding  innerPadding  separator  split  iconInline
buttonRadius  cardRadius  dialogRadius  inputRadius  popupRadius
menuRadius  selectionRadius  scrollBarRadius  inputBorder
scrollBar  scrollBarSmall  modalBlurRadius
innerWindowRadius  windowTitleBarHeight  windowButtonHeight
windowButtonRadius  windowButtonIcon
```

**Fonts** and **Icons** are maps of name to file URI. **99 built-in icon names** can be
replaced wholesale — `confirm`, `cancel`, `error`, `warning`, `info`, `question`,
`checked`, `unchecked`, `arrowDown`, `moreVertical`, and so on — so the stock icon set
can be swapped for yours without touching Go. Fonts fill six slots: regular, bold,
italic, bold-italic, monospace, and symbol.

Beyond the theme file:

- **`widget.Importance`** is the main per-instance lever: `Low`, `Medium`, `High`,
  `Danger`, `Warning`, `Success`. It binds a widget to the semantic colours. This is how
  a button becomes destructive or a status becomes a warning.
- **`container.NewThemeOverride`** scopes a different theme to one subtree, so per-region
  theming is possible where it earns its keep.
- **Shadows** are real and fully specified: colour, blur radius, spread, offset, and drop
  versus box variant, on rectangles, circles, and ellipses.
- **`canvas.Blur`** is a blur region that affects everything drawn beneath it.
- **Gradients**, linear and radial.
- **Animation** covers colour, position, size, and shaders, with easing.
- **Canvas primitives** are richer than a widget toolkit usually offers: rectangle with
  corner radius and stroke, circle, ellipse, line, arc, bézier curve, polygon, regular
  polygon, arbitrary polygon, text, image, raster, and shader.

### What you cannot do

- **No CSS.** Layout is a Go container tree. The available containers are Border, HBox,
  VBox, Grid by columns or rows, adaptive Grid, GridWrap, Stack, Scroll, Split, Padded,
  Center, Clip, AppTabs, DocTabs, and Navigation — plus a hand-written layout if none
  fit. There is no flexbox and no grid-template semantics. Designs that depend on
  arbitrary reflow will be expensive.
- **The widget set is fixed.** Button, Label, Entry, Check, CheckGroup, RadioGroup,
  Select, SelectEntry, Slider, ProgressBar, ProgressBarInfinite, Activity, List, Table,
  Tree, GridWrap, Accordion, Card, Form, Toolbar, Separator, Icon, Hyperlink, RichText,
  Markdown, TextGrid, Menu, Popup, Calendar, DateEntry. Anything else is a hand-written
  Go renderer. That is possible and sometimes right, but each one is real engineering
  cost — so propose them individually with a reason, not as a component library.
- **Stock widgets do not expose a shadow property.** Shadowed surfaces mean drawing a
  rectangle behind widget content, which is cheap composition rather than a custom
  widget, but it is not free either.
- **A custom font costs binary size.** The GUI binary is already about 41 MB, because
  the rendering stack is compiled in rather than borrowed from the OS. Fyne bundles its
  own faces; replacing them means supplying files for as many of the six slots as you
  intend to change, and stating the size cost rather than leaving it to be discovered.
- **Accessibility is weaker than a native or webview UI.** ADR-0005 concedes this
  explicitly. It means the design has to carry more of the load: **never colour as the
  sole signal**, contrast met in both light and dark rather than one checked and the
  other eyeballed, and generous hit targets.
- **The window is 900×640 today**, and "large enough for a provider list beside a plan"
  is a comment in `internal/gui/app.go`. It is the only layout decision anyone has made
  and it is not defended. A better answer is welcome.

### The terminal

The CLI has its own constraints, and they are less forgiving than they look.

- **Windows consoles will mangle Unicode.** Legacy code pages in `conhost` are still out
  there. An **ASCII fallback glyph set is required**, paired one-to-one with the Unicode
  set. This is not a nice-to-have.
- **Colour must be removable without the layout changing.** `NO_COLOR`, piped output, and
  CI all have to work. If a state is only distinguishable by colour, it disappears in
  half the places this tool runs.
- **`--json` emits nothing else on stdout.** Styling and content have to be separable.
- **Column widths must survive** an 80-column terminal and package names longer than the
  column.
- **All styling lives in `internal/cli`.** Nothing below it may emit a colour code —
  that boundary is the rule that lets the GUI share the core, and it is enforced.
- **There is no styling library in the project today.** The dependencies are Fyne and
  Cobra, and nothing else. Adding one is a decision this project settles with a decision
  record. So: **specify the colours, glyphs, and layout you need; the library that
  produces them is engineering's call.** A design expressed as "these sixteen ANSI
  colours, these glyph pairs, these column rules" is implementable. A design expressed
  as a particular library's API is not portable into that decision.

---

## What we are asking for

Split by how long it will stay correct, because the GUI is nine milestones away.

### Durable — commission now, will still be right at M11

1. **App icon.** An unmet M12 requirement; nothing exists today. Needs to work at
   taskbar size on Windows and Linux, in both light and dark surroundings.
2. **`theme.json`** — the complete token set, with light and dark **both authored**.
   Every value must map to a real Fyne token name from the lists above.
3. **The size system** with its rationale: type scale across the four text sizes,
   spacing from `padding` and `innerPadding`, and a coherent radius story across the
   eight radius tokens.
4. **Font decision.** Keep Fyne's bundled Inter and Noto Sans, or supply a family with
   the binary-size cost stated.
5. **Icon set** as SVG at one stroke weight, covering the provider states, the actions,
   and whichever built-in Fyne icon names you are replacing.
6. **The shared state vocabulary** — the single highest-value deliverable. Every
   provider state and every failure kind from the tables above, given a colour, a glyph,
   and a word, defined once and expressed in both media. This is what makes the two
   frontends one product.
7. **Terminal design**: the palette, the Unicode and ASCII glyph pairs, and the column
   layout for the plan and the run summary.

### Provisional — expect revision

Drawn against a data model that does not exist yet, so treat these as direction rather
than specification.

- **M11 screens**: provider list with detected status and elevation requirement; the
  plan grouped by provider; progress during detect and plan; a provider that fails
  during plan showing as failed in place without taking down the window.
- **M12 screens**: apply with per-provider live status; the confirmation dialog;
  elevation prompts; config editing; run history; cancel mid-apply.

### Out of scope

- Anything implying a non-goal: scheduling UI, rollback affordances, remote or
  multi-machine views, inventory.
- A marketing site or a documentation site. [ADR-0007](../adr/0007-godoc-as-reference-documentation.md)
  declined the latter deliberately.

---

## What makes it usable

Written to be checkable, in the style the roadmap uses. If a criterion can be argued
about, it is written wrong.

- [ ] Every colour in the design maps to one of the 30 Fyne colour token names.
- [ ] Every size maps to one of the 27 Fyne size token names.
- [ ] The theme loads via `theme.FromJSON` and needs no Go changes to apply.
- [ ] Light and dark are both complete, and both pass contrast checks independently.
- [ ] Every component maps to a named Fyne widget, or the design states what the custom
      renderer costs and why it is worth it.
- [ ] Every provider state and every failure kind has a defined treatment in **both**
      the GUI and the terminal.
- [ ] No state is distinguished by colour alone, in either medium.
- [ ] The terminal design is legible with colour disabled and with ASCII glyphs only.
- [ ] Elevation is visible in the plan before the confirmation, in both frontends.
- [ ] The partial-success run — some succeeded, one failed, one blocked — is designed,
      not left to fall out of the all-green case.
- [ ] Nothing in the design requires a capability listed as a non-goal.

---

## Where to look

| | |
|---|---|
| [README.md](../README.md) | The pitch, and a worked example of the CLI output |
| [ARCHITECTURE.md](ARCHITECTURE.md) | The pipeline, the error taxonomy, exit codes, the frontend boundary |
| [ROADMAP.md](ROADMAP.md) | M11 and M12 are the GUI; the acceptance criteria there are binding |
| [ADR-0005](../adr/0005-fyne-for-the-gui-client.md) | Why Fyne, what it costs, what was rejected |
| [ADR-0003](../adr/0003-per-provider-elevation.md) | Why elevation is per-provider and visible |
| [`internal/gui/doc.go`](../internal/gui/doc.go) | The containment rule for Fyne code |
| [`.github/ISSUE_TEMPLATE/`](../.github/ISSUE_TEMPLATE/) | The only place real user-facing vocabulary already exists |
