package main

import (
	"strings"
	"testing"
)

func TestParseSimpleRecipe(t *testing.T) {
	input := `# Build the project
build:
	go build ./...
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jf.Recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(jf.Recipes))
	}

	r := jf.Recipes[0]
	assertEqual(t, "name", r.Name, "build")
	assertEqual(t, "doc", r.Doc, "Build the project")
	assertEqual(t, "lines count", len(r.Lines), 1)
	assertEqual(t, "line 0", r.Lines[0], "go build ./...")
}

func TestParseVariableAssignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantVal  string
		export   bool
		backtick bool
	}{
		{
			name:     "simple string",
			input:    `name := "hello"`,
			wantName: "name",
			wantVal:  "hello",
		},
		{
			name:     "unquoted value",
			input:    `count := 42`,
			wantName: "count",
			wantVal:  "42",
		},
		{
			name:     "export variable",
			input:    `export PATH := "/usr/bin"`,
			wantName: "PATH",
			wantVal:  "/usr/bin",
			export:   true,
		},
		{
			name:     "backtick command",
			input:    "version := `git describe --tags`",
			wantName: "version",
			wantVal:  "git describe --tags",
			backtick: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(jf.Variables) != 1 {
				t.Fatalf("expected 1 variable, got %d", len(jf.Variables))
			}

			v := jf.Variables[0]
			assertEqual(t, "name", v.Name, tt.wantName)
			assertEqual(t, "value", v.Value, tt.wantVal)
			assertEqual(t, "export", v.Export, tt.export)
			assertEqual(t, "backtick", v.Backtick, tt.backtick)
		})
	}
}

func TestParseRecipeWithParams(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantParams []Param
	}{
		{
			name: "positional param",
			input: `greet name:
	echo "hello {{name}}"
`,
			wantParams: []Param{{Name: "name"}},
		},
		{
			name: "variadic star",
			input: `cli *ARGS:
	cargo run -- {{ARGS}}
`,
			wantParams: []Param{{Name: "ARGS", Variadic: "*"}},
		},
		{
			name: "variadic plus",
			input: `run +FILES:
	cat {{FILES}}
`,
			wantParams: []Param{{Name: "FILES", Variadic: "+"}},
		},
		{
			name: "default value",
			input: `deploy env="staging":
	echo deploying to {{env}}
`,
			wantParams: []Param{{Name: "env", Default: "staging"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(jf.Recipes) != 1 {
				t.Fatalf("expected 1 recipe, got %d", len(jf.Recipes))
			}

			r := jf.Recipes[0]
			if len(r.Params) != len(tt.wantParams) {
				t.Fatalf("expected %d params, got %d", len(tt.wantParams), len(r.Params))
			}

			for i, want := range tt.wantParams {
				got := r.Params[i]
				assertEqual(t, "param name", got.Name, want.Name)
				assertEqual(t, "param variadic", got.Variadic, want.Variadic)
				assertEqual(t, "param default", got.Default, want.Default)
			}
		})
	}
}

func TestParseRecipeWithDeps(t *testing.T) {
	input := `all: build test lint
	echo "done"
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := jf.Recipes[0]
	wantDeps := []string{"build", "test", "lint"}
	if len(r.Dependencies) != len(wantDeps) {
		t.Fatalf("expected %d deps, got %d", len(wantDeps), len(r.Dependencies))
	}
	for i, want := range wantDeps {
		assertEqual(t, "dep", r.Dependencies[i], want)
	}
}

func TestParseAlias(t *testing.T) {
	input := `alias b := build

# Build
build:
	go build
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jf.Aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(jf.Aliases))
	}
	assertEqual(t, "alias name", jf.Aliases[0].Name, "b")
	assertEqual(t, "alias target", jf.Aliases[0].Target, "build")
}

func TestParseSectionSeparatorsNotDoc(t *testing.T) {
	input := `# --- Build ---

# Compile the binary
build:
	go build
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := jf.Recipes[0]
	assertEqual(t, "doc", r.Doc, "Compile the binary")
}

func TestParseDefaultListRecipe(t *testing.T) {
	input := `# Default recipe - show available commands
default:
	@just --list
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := jf.Recipes[0]
	if !isListDefault(&r) {
		t.Error("expected default recipe to be detected as list default")
	}
}

func TestParseBrainiacJustfile(t *testing.T) {
	input := `# Brainiac monorepo development commands

# Default recipe - show available commands
default:
    @just --list

# --- Desktop App ---

# Run the desktop app in development mode
dev:
    npm run tauri -w @brainiac/desktop -- dev

# Build the desktop app for release
build:
    npm run tauri -w @brainiac/desktop -- build

# Build without bundling (faster, just creates the binary)
build-fast:
    npm run tauri -w @brainiac/desktop -- build -- --no-bundle

# Build only the frontend (Vite)
build-frontend:
    npm run build -w @brainiac/desktop

# --- Server & CLIs ---

# Run the HTTP API server
server:
    cargo run -p brainiac-server

# Run the CLI agent
cli *ARGS:
    cargo run -p brainiac-cli -- {{ARGS}}

# Run the E2E test harness
tester *ARGS:
    cargo run -p brainiac-tester -- {{ARGS}}

# --- Rust workspace ---

# Check Rust code without building
check:
    cargo check --workspace

# Run Rust tests
test:
    cargo test --workspace

# Format Rust code
fmt:
    cargo fmt --all

# Lint Rust code
clippy:
    cargo clippy --workspace -- -D warnings

# --- TypeScript ---

# Type-check all TypeScript packages
typecheck:
    npx tsc --noEmit -p packages/shared
    npx tsc --noEmit -p apps/desktop

# --- Utilities ---

# Clean build artifacts
clean:
    cargo clean
    rm -rf apps/desktop/dist

# Install dependencies
install:
    npm install

# Reinstall all dependencies (clean install)
reinstall:
    rm -rf node_modules apps/desktop/node_modules apps/mobile/node_modules packages/shared/node_modules package-lock.json Cargo.lock
    npm install

# Add a shadcn component to the desktop app
shadcn *ARGS:
    npx shadcn@latest {{ARGS}} --cwd apps/desktop

# Open the built app (macOS)
open:
    open target/release/bundle/macos/Brainiac.app
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected recipe count (default + 17 actual recipes).
	expectedRecipes := 18
	if len(jf.Recipes) != expectedRecipes {
		t.Fatalf("expected %d recipes, got %d", expectedRecipes, len(jf.Recipes))
	}

	// Check default recipe is list.
	if !isListDefault(&jf.Recipes[0]) {
		t.Error("expected first recipe to be list default")
	}

	// Check a recipe with variadic params.
	cli := findTestRecipe(t, jf, "cli")
	if len(cli.Params) != 1 {
		t.Fatalf("cli: expected 1 param, got %d", len(cli.Params))
	}
	assertEqual(t, "cli param name", cli.Params[0].Name, "ARGS")
	assertEqual(t, "cli param variadic", cli.Params[0].Variadic, "*")
	assertEqual(t, "cli doc", cli.Doc, "Run the CLI agent")
	assertEqual(t, "cli lines", len(cli.Lines), 1)
	assertEqual(t, "cli body", cli.Lines[0], "cargo run -p brainiac-cli -- {{ARGS}}")

	// Check multi-line recipe.
	typecheck := findTestRecipe(t, jf, "typecheck")
	assertEqual(t, "typecheck lines", len(typecheck.Lines), 2)

	reinstall := findTestRecipe(t, jf, "reinstall")
	assertEqual(t, "reinstall lines", len(reinstall.Lines), 2)

	// Section separators should not become doc comments.
	dev := findTestRecipe(t, jf, "dev")
	assertEqual(t, "dev doc", dev.Doc, "Run the desktop app in development mode")
}

func TestConvertLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "interpolation",
			input: "echo {{NAME}}",
			want:  "echo $(NAME)",
		},
		{
			name:  "backtick in body",
			input: "echo `git describe`",
			want:  "echo $(shell git describe)",
		},
		{
			name:  "mixed",
			input: "deploy {{ENV}} `date`",
			want:  "deploy $(ENV) $(shell date)",
		},
		{
			name:  "no conversion needed",
			input: "go build ./...",
			want:  "go build ./...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertLine(tt.input)
			assertEqual(t, "converted line", got, tt.want)
		})
	}
}

func TestGenerateBrainiacMakefile(t *testing.T) {
	input := `# Default recipe - show available commands
default:
    @just --list

# Build the project
build:
    go build ./...

# Run tests
test:
    go test ./...

# Run the CLI
cli *ARGS:
    go run . -- {{ARGS}}
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := Generate(jf, true)

	// Check header.
	if !strings.Contains(output, "# Generated by jmake") {
		t.Error("missing header comment")
	}
	if !strings.Contains(output, "SHELL := /bin/bash") {
		t.Error("missing SHELL assignment")
	}

	// Check .PHONY includes help.
	if !strings.Contains(output, ".PHONY:") {
		t.Error("missing .PHONY")
	}
	if !strings.Contains(output, "help") {
		t.Error("missing help in .PHONY")
	}

	// Check help target exists.
	if !strings.Contains(output, "help:") {
		t.Error("missing help target")
	}

	// Check recipe conversion.
	if !strings.Contains(output, "build:") {
		t.Error("missing build target")
	}
	if !strings.Contains(output, "\tgo build ./...") {
		t.Error("missing build command")
	}

	// Check interpolation conversion.
	if !strings.Contains(output, "$(ARGS)") {
		t.Error("{{ARGS}} not converted to $(ARGS)")
	}

	// Default recipe (just --list) should not appear.
	if strings.Contains(output, "default:") {
		t.Error("default recipe should have been replaced by help")
	}
}

func TestListRecipes(t *testing.T) {
	input := `default:
    @just --list

# Build it
build:
    go build

# Run with args
run *ARGS:
    go run . {{ARGS}}
`

	jf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ListRecipes(jf)

	if !strings.Contains(output, "Available recipes:") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "build") {
		t.Error("missing build recipe")
	}
	if !strings.Contains(output, "Build it") {
		t.Error("missing build doc")
	}
	if !strings.Contains(output, "*ARGS") {
		t.Error("missing variadic param display")
	}
	// Default list recipe should be excluded.
	if strings.Contains(output, "default") {
		t.Error("default list recipe should be excluded from listing")
	}
}

func TestMapArgs(t *testing.T) {
	tests := []struct {
		name    string
		recipe  Recipe
		args    []string
		want    []string
		wantErr bool
	}{
		{
			name: "positional args",
			recipe: Recipe{
				Name:   "deploy",
				Params: []Param{{Name: "env"}, {Name: "tag"}},
			},
			args: []string{"prod", "v1.0"},
			want: []string{"env=prod", "tag=v1.0"},
		},
		{
			name: "variadic star collects all",
			recipe: Recipe{
				Name:   "cli",
				Params: []Param{{Name: "ARGS", Variadic: "*"}},
			},
			args: []string{"--verbose", "run"},
			want: []string{"ARGS=--verbose run"},
		},
		{
			name: "variadic star no args is ok",
			recipe: Recipe{
				Name:   "cli",
				Params: []Param{{Name: "ARGS", Variadic: "*"}},
			},
			args: nil,
			want: nil,
		},
		{
			name: "variadic plus requires at least one",
			recipe: Recipe{
				Name:   "run",
				Params: []Param{{Name: "FILES", Variadic: "+"}},
			},
			args:    nil,
			wantErr: true,
		},
		{
			name: "missing required arg",
			recipe: Recipe{
				Name:   "deploy",
				Params: []Param{{Name: "env"}},
			},
			args:    nil,
			wantErr: true,
		},
		{
			name: "default value used when no arg",
			recipe: Recipe{
				Name:   "deploy",
				Params: []Param{{Name: "env", Default: "staging"}},
			},
			args: nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapArgs(&tt.recipe, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("expected %d assignments, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range got {
				assertEqual(t, "assignment", got[i], tt.want[i])
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   options
	}{
		{
			name: "no args",
			args: nil,
			want: options{},
		},
		{
			name: "list flag",
			args: []string{"--list"},
			want: options{list: true},
		},
		{
			name: "short list flag",
			args: []string{"-l"},
			want: options{list: true},
		},
		{
			name: "dump flag",
			args: []string{"--dump"},
			want: options{dump: true},
		},
		{
			name: "target only",
			args: []string{"build"},
			want: options{target: "build", args: []string{}},
		},
		{
			name: "target with args",
			args: []string{"cli", "hello", "world"},
			want: options{target: "cli", args: []string{"hello", "world"}},
		},
		{
			name: "file flag then target",
			args: []string{"-f", "myfile", "build"},
			want: options{justfilePath: "myfile", target: "build", args: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseArgs(tt.args)
			assertEqual(t, "justfilePath", got.justfilePath, tt.want.justfilePath)
			assertEqual(t, "list", got.list, tt.want.list)
			assertEqual(t, "dump", got.dump, tt.want.dump)
			assertEqual(t, "dryRun", got.dryRun, tt.want.dryRun)
			assertEqual(t, "showHelp", got.showHelp, tt.want.showHelp)
			assertEqual(t, "showVersion", got.showVersion, tt.want.showVersion)
			assertEqual(t, "target", got.target, tt.want.target)

			if tt.want.args != nil {
				if len(got.args) != len(tt.want.args) {
					t.Fatalf("expected %d args, got %d", len(tt.want.args), len(got.args))
				}
				for i := range got.args {
					assertEqual(t, "arg", got.args[i], tt.want.args[i])
				}
			}
		})
	}
}

// findTestRecipe is a test helper that finds a recipe by name.
func findTestRecipe(t *testing.T, jf *Justfile, name string) *Recipe {
	t.Helper()
	for i := range jf.Recipes {
		if jf.Recipes[i].Name == name {
			return &jf.Recipes[i]
		}
	}
	t.Fatalf("recipe %q not found", name)
	return nil
}

// assertEqual is a generic test helper for comparing values.
func assertEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}
