# jmake

Run [justfile](https://github.com/casey/just) recipes via `make` -- no `just` installation required.

`jmake` parses a justfile, generates a temporary Makefile, and executes it. It also supports `--dump` to output a standalone Makefile.

## Install

Requires Go 1.24+.

```sh
go install github.com/sammcj/jmake@latest
```

Or build from source:

```sh
make build        # outputs to ./bin/jmake
make install      # copies to $GOPATH/bin or /usr/local/bin
```

## Usage

```sh
jmake                     # run default recipe (or list recipes if default calls just --list)
jmake build               # run the "build" recipe
jmake deploy prod v1.2    # positional args mapped to recipe parameters
jmake -l                  # list available recipes
jmake -d                  # print generated Makefile to stdout
jmake -n build            # dry run -- show the make command without executing
jmake -f path/justfile    # use a specific justfile
```

### Flags

| Flag          | Short | Description                         |
| ------------- | ----- | ----------------------------------- |
| `--list`      | `-l`  | List available recipes              |
| `--dump`      | `-d`  | Print generated Makefile to stdout  |
| `--file PATH` | `-f`  | Specify justfile path               |
| `--dry-run`   | `-n`  | Show make command without executing |
| `--help`      | `-h`  | Show help                           |
| `--version`   | `-v`  | Show version                        |

## Supported justfile features

- Recipes with commands, doc comments, and dependencies
- Parameters: positional, variadic (`*ARGS`, `+ARGS`), defaults (`name="val"`)
- Variable assignments (`name := "value"`)
- Backtick expressions (`` `cmd` `` becomes `$(shell cmd)`)
- `export` variables
- `{{VAR}}` interpolation (becomes `$(VAR)`)
- `@` silent prefix
- Aliases (`alias name := target`)
- `@just --list` in default recipe detected and replaced with native listing

## Conversion reference

| Justfile        | Makefile                  |
| --------------- | ------------------------- |
| `{{VAR}}`       | `$(VAR)`                  |
| `` `cmd` ``     | `$(shell cmd)`            |
| `name := "val"` | `name := val`             |
| `export X := Y` | `export X := Y`           |
| `@command`      | `@command`                |
| recipe params   | `make target PARAM=value` |
| recipe deps     | target prerequisites      |

## Justfile discovery

`jmake` searches the current directory and parent directories for files named `justfile`, `Justfile`, or `.justfile`.

## Development

```sh
make build       # build binary
make test        # run tests with race detector
make lint        # run golangci-lint
make modernize   # run gopls modernize checks
make clean       # remove build artefacts
```

## Licence

MIT
