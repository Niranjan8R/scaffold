# scaffold

A fast, dependency-free CLI (Go, standard library only) that generates
ready-to-build project skeletons for performance-oriented work:

| Template       | What you get |
|----------------|--------------|
| `cpp-cmake`    | Library + app + tests, CMake, CTest, GoogleTest or Catch2 (fetched with `FetchContent`) |
| `cuda`         | Host/kernel split, `CUDA_CHECK` macro, event timing, CPU verification, configurable GPU arch |
| `embedded-avr` | `avr-gcc` Makefile, CMake **cross-compilation toolchain file**, `.hex` output, `avrdude` flash target |

You can also create your own templates and load them using
the `--template-dir` flag.

## Install

```sh
go install github.com/Niranjan8R/scaffold@latest
```

or build from source: `make build` (binary in `bin/scaffold`).

## Usage

```sh
scaffold list
scaffold new                                   # interactive
scaffold new cpp-cmake my-lib --var Tests=catch2 --var Std=20
scaffold new cuda vecadd --var Arch=86 --yes   # non-interactive (CI friendly)
scaffold new embedded-avr blinky --dir ~/firmware
```

Flags: `--dir`, `--template-dir`, `--var key=value`, `--yes`, `--force`.
Existing files are never overwritten unless you pass `--force`, and nothing is
written if any file would conflict.

## Custom templates

A template is a folder with a `template.json` and files:

```
my-template/
  template.json            # name, description, vars, next-steps text
  README.md.tmpl           # *.tmpl  -> rendered with text/template, suffix dropped
  src/__NameIdent__.c.tmpl # __Var__ in names is replaced by the variable's value
  LICENSE                  # anything else is copied verbatim
```

Always-available variables: `Name`, `NameIdent` (`my-app` -> `my_app`),
`NameUpper` (`MY_APP`). Declare more in `template.json`:

```json
{"name": "demo", "description": "...",
 "vars": [{"name": "Std", "prompt": "C++ standard", "default": "17", "choices": ["14","17","20"]}]}
```

See `examples/templates/python-pkg` and run
`scaffold new python-pkg x --template-dir examples/templates`.

## Design

- `internal/engine`: loads a template, validates variables, renders all output in
  memory first (atomic: no half-generated projects), blocks path traversal.
- `internal/templates`: built-ins via `go:embed` (single static binary), user templates via `os.DirFS`.
- `main.go`: subcommands, flag parsing that allows flags anywhere, interactive prompts.
- Tests cover the engine, every built-in template (rendered output has no leftover
  `{{`), interactive mode, error paths and custom templates.
- CI builds the generated C++ project, runs its tests and compiles the AVR firmware.


## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.