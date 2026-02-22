# OpenBindings CLI (`ob`)

The OpenBindings CLI (`ob`) is a command-line tool for creating, browsing, syncing, and validating OpenBindings Interface (OBI) documents.

## Install

Build the latest local `ob` and link it into your user PATH:

```bash
bash cli/scripts/dev-install.sh
```

Defaults to `~/.local/bin`. Override with `OB_BIN_DIR="$HOME/bin"`.

## Quick Start

```bash
# Create an OBI from a usage spec
ob create usage@2.0.0:./cli.kdl -o interface.json

# Browse and interact with an OBI
ob browse interface.json

# Check drift status
ob status interface.json

# Sync sources to update the OBI
ob sync interface.json
```

## Core Concepts

### OpenBindings Interface (OBI)

An OBI is a JSON document that describes a software interface: its operations, the binding sources that implement those operations, and the bindings that connect them. OBIs are format-agnostic — they work with any binding specification (OpenAPI, usage spec, protobuf, etc.).

### Sources

A **source** is a reference to a binding specification artifact. Each source has a `format` (e.g., `usage@2.0.0`, `openapi@3.1`) and either a `location` (path/URI) or `content` (embedded data).

### Operations and Bindings

**Operations** are what your interface can do (methods, events). **Bindings** map operations to specific references within sources.

### x-ob Metadata

`ob` uses `x-ob` extension fields to track management metadata on OBI objects. This is `ob`'s private workspace — other tools ignore it.

- **On sources**: `x-ob` contains `ref` (where to fetch from), `resolve` (how to populate spec fields), `contentHash` (for drift detection), `lastSynced`, and `obVersion`.
- **On operations/bindings**: `x-ob: {}` means "managed by ob" — sync can overwrite it. Absence means hand-authored — sync leaves it alone.

The OBI is always spec-valid. Stripping `x-ob` is optional cosmetic cleanup.

## How ob Works: Delegates

`ob` uses the **delegate pattern** to keep its core small and extensible. Instead of building format-specific logic or credential management into the core, `ob` delegates these responsibilities to software that implements standard OpenBindings interface contracts.

A **delegate** is any software that:

1. Exposes an OpenBindings interface document (via `--openbindings` flag, `/.well-known/openbindings` endpoint, or direct reference)
2. Imports and satisfies one or more standard delegate interfaces

`ob` currently defines two delegate interfaces:

### Binding Format Handlers

A **binding format handler** interprets a specific binding format (e.g., OpenAPI, MCP, usage spec) and knows how to create interfaces from binding sources, list supported formats, and execute operations via bindings. Defined by `openbindings.binding-format-handler.json`.

### Binding Context Providers

A **binding context provider** produces runtime context needed to execute a binding — credentials, headers, cookies, environment variables, and other metadata. Defined by `openbindings.binding-context-provider.json`.

### Execution Flow

When `ob` executes an operation, the flow is:

```
User
  |
  v
ob (orchestrator)
  |
  |-- getContext(source, ref) --> Binding Context Provider
  |                                   |
  |   <-- { context } ----------------+
  |
  |-- executeOperation(source, ref, input, context) --> Binding Format Handler
  |                                                         |
  |   <-- { output } --------------------------------------+
  |
  v
Result returned to user
```

The context provider tells `ob` *how to reach* the service (e.g., "use this bearer token"). The format handler tells `ob` *how to talk to* the service (e.g., "this is an OpenAPI endpoint, here's the HTTP request"). `ob` coordinates both without knowing the details of either.

### ob as a Delegate

`ob` itself implements both delegate interfaces. It serves as the built-in format handler (supporting usage spec, MCP, and more) and can serve as a built-in context provider. Because `ob` properly declares these interfaces in its own OBI, any alternative orchestrator can use `ob` as a drop-in delegate.

### Extensibility

New delegate types can be added by defining new OBI contracts. The pattern is always the same: define an interface, import `openbindings.software.json` for identity, and declare the operations the delegate must implement. Third-party delegates register with `ob delegate add` and are discovered dynamically.

## Source Resolution Modes

When adding a source, `--resolve` controls how it appears in the OBI:

### `location` (default)

The spec `location` field stores a path or URI to the source artifact.

```bash
ob source add interface.json usage@2.0.0:./cli.kdl
```

Produces:
```json
{
  "format": "usage@2.0.0",
  "location": "./cli.kdl",
  "x-ob": { "ref": "./cli.kdl", "resolve": "location", "contentHash": "sha256:..." }
}
```

With `--uri`, the spec location uses the given URI instead of the local path:

```bash
ob source add interface.json openapi@3.1:./api.yaml --uri https://cdn.example.com/api.yaml
```

### `content`

The source content is embedded directly in the OBI.

```bash
ob source add interface.json usage@2.0.0:./cli.kdl --resolve content
```

Produces:
```json
{
  "format": "usage@2.0.0",
  "content": "min_usage_version \"2.0.0\"\nbin \"hello\"...",
  "x-ob": { "ref": "./cli.kdl", "resolve": "content", "contentHash": "sha256:..." }
}
```

JSON/YAML formats embed as native objects. Text formats (KDL, protobuf) embed as strings.

## Drift Detection

`ob` computes a SHA-256 hash of each source artifact at sync time and stores it in `x-ob.contentHash`. Later, `ob status` compares the current file against the stored hash to detect changes.

```bash
ob status interface.json
```

```
greet v1.0.0  (openbindings 0.1.0)

Sources (3)
  usage          usage@2.0.0    ./usage.kdl          current (synced 2d ago, ob 0.1.0)
  configSpec     usage@2.0.0    ./config.usage.kdl   drifted (synced 5d ago, ob 0.1.0)
  toolsSpec      usage@2.0.0    ./tools.usage.kdl    drifted (synced 5d ago, ob 0.1.0)

Operations (6)  — 6 managed, 0 hand-authored
Bindings (6)  — 6 managed, 0 hand-authored

2 source(s) drifted. Run 'ob sync interface.json' to update.
```

## Syncing

`ob sync` re-reads sources from their `x-ob.ref` locations and updates the OBI.

```bash
# Full sync — all sources
ob sync interface.json

# Partial sync — just named sources
ob sync interface.json usage configSpec

# Sync and write to a different file
ob sync interface.json -o dist/interface.json

# Sync and strip x-ob metadata for publishing
ob sync interface.json -o published.json --pure
```

### `--pure`

Strips all `x-ob` metadata from the output, producing a clean spec-only OBI suitable for distribution. Requires `-o` to prevent accidental in-place metadata loss.

## Managed vs. Hand-Authored

- **Managed**: objects with `x-ob` present. Created by `ob create` or `ob source add`. Sync can overwrite them.
- **Hand-authored**: objects without `x-ob`. Added manually by the user. Sync never touches them.

To detach an object from sync management, remove its `x-ob` field. The object stays in the OBI but sync stops managing it.

## Command Reference

### Interface Authoring

| Command | Description |
|---------|-------------|
| `ob create <sources...>` | Create an OBI from binding source artifacts |
| `ob source add <obi> <format:path>` | Register a source reference |
| `ob source list <obi>` | List source references |
| `ob source remove <obi> <key>` | Remove a source reference |
| `ob sync <obi> [sources...]` | Sync sources from x-ob references |
| `ob diff <obi>` | Show differences between OBI and sources |
| `ob merge <obi>` | Merge source changes into OBI |

### Exploration

| Command | Description |
|---------|-------------|
| `ob browse [target]` | Browse and interact with targets (TUI) |
| `ob execute <obi>` | Execute an operation from an OBI |
| `ob status [obi]` | Show environment status or OBI drift report |

### Workspace Management

| Command | Description |
|---------|-------------|
| `ob init` | Initialize an OpenBindings environment |
| `ob workspace list` | List workspaces |
| `ob workspace create <name>` | Create a workspace |
| `ob target add <url>` | Add a target to the active workspace |

### Introspection

| Command | Description |
|---------|-------------|
| `ob validate <obi>` | Validate an OBI document |
| `ob compat <a> <b>` | Check compatibility between OBIs |
| `ob formats` | List format tokens this ob instance can handle |

## Design Documents

- `DESIGN.md` — Architecture and invariants
- `PLAN.md` — Roadmap and sequencing
- `design/commands/sync.md` — Sync command internal design
