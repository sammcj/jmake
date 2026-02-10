package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "jmake: %s\n", err)
		os.Exit(1)
	}
}

type options struct {
	justfilePath string
	list         bool
	dump         bool
	dryRun       bool
	showHelp     bool
	showVersion  bool
	target       string
	args         []string
}

func parseArgs(args []string) options {
	var opts options

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--file" || a == "-f":
			i++
			if i < len(args) {
				opts.justfilePath = args[i]
			}
		case a == "--list" || a == "-l":
			opts.list = true
		case a == "--dump" || a == "-d":
			opts.dump = true
		case a == "--dry-run" || a == "-n":
			opts.dryRun = true
		case a == "--help" || a == "-h":
			opts.showHelp = true
		case a == "--version" || a == "-v":
			opts.showVersion = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "jmake: unknown flag: %s\n", a)
			os.Exit(1)
		default:
			// First non-flag is the target, rest are recipe args.
			opts.target = a
			opts.args = args[i+1:]
			return opts
		}
		i++
	}
	return opts
}

func run(args []string) error {
	opts := parseArgs(args)

	if opts.showHelp {
		printUsage()
		return nil
	}
	if opts.showVersion {
		fmt.Printf("jmake %s\n", version)
		return nil
	}

	justfilePath := opts.justfilePath
	if justfilePath == "" {
		var err error
		justfilePath, err = findJustfile()
		if err != nil {
			return err
		}
	}

	f, err := os.Open(justfilePath)
	if err != nil {
		return fmt.Errorf("opening justfile: %w", err)
	}
	defer f.Close()

	jf, err := Parse(f)
	if err != nil {
		return err
	}

	// Determine if the default recipe is a `just --list` wrapper.
	hasListDefault := len(jf.Recipes) > 0 && isListDefault(&jf.Recipes[0])

	// --list: print recipes and exit.
	if opts.list {
		fmt.Print(ListRecipes(jf))
		return nil
	}

	// --dump: generate and print Makefile, then exit.
	if opts.dump {
		fmt.Print(Generate(jf, hasListDefault))
		return nil
	}

	// No target specified: if default is list, show list; otherwise use default.
	if opts.target == "" {
		if hasListDefault {
			fmt.Print(ListRecipes(jf))
			return nil
		}
		if len(jf.Recipes) > 0 {
			opts.target = jf.Recipes[0].Name
		} else {
			return fmt.Errorf("no recipes found in justfile")
		}
	}

	// Resolve aliases.
	opts.target = resolveAlias(jf, opts.target)

	// Find the target recipe.
	recipe := findRecipe(jf, opts.target)
	if recipe == nil {
		return fmt.Errorf("unknown recipe: %s", opts.target)
	}

	// Build make variable assignments from positional args.
	makeVars, err := mapArgs(recipe, opts.args)
	if err != nil {
		return err
	}

	// Generate Makefile.
	content := Generate(jf, hasListDefault)

	// Write to temp file and execute make.
	tmpFile, err := os.CreateTemp("", "jmake-*.mk")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp makefile: %w", err)
	}
	tmpFile.Close()

	// Build make command.
	makeArgs := []string{"--no-print-directory", "-f", tmpPath, opts.target}
	makeArgs = append(makeArgs, makeVars...)

	if opts.dryRun {
		fmt.Printf("make %s\n", strings.Join(makeArgs, " "))
		return nil
	}

	cmd := exec.Command("make", makeArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set working directory to the justfile's directory.
	cmd.Dir = filepath.Dir(justfilePath)

	return cmd.Run()
}

// findJustfile searches for a justfile starting from cwd and walking up.
func findJustfile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	names := []string{"justfile", "Justfile", ".justfile"}

	for {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no justfile found")
}

// resolveAlias resolves an alias to its target recipe name.
func resolveAlias(jf *Justfile, name string) string {
	for _, a := range jf.Aliases {
		if a.Name == name {
			return a.Target
		}
	}
	return name
}

// findRecipe returns the recipe with the given name, or nil.
func findRecipe(jf *Justfile, name string) *Recipe {
	for i := range jf.Recipes {
		if jf.Recipes[i].Name == name {
			return &jf.Recipes[i]
		}
	}
	return nil
}

// mapArgs maps positional CLI args to recipe parameters, returning Make variable assignments.
func mapArgs(r *Recipe, args []string) ([]string, error) {
	var assignments []string

	argIdx := 0
	for _, p := range r.Params {
		if p.Variadic != "" {
			// Collect all remaining args.
			if p.Variadic == "+" && argIdx >= len(args) {
				return nil, fmt.Errorf("recipe '%s' requires at least one argument for '%s'", r.Name, p.Name)
			}
			if argIdx < len(args) {
				val := strings.Join(args[argIdx:], " ")
				assignments = append(assignments, fmt.Sprintf("%s=%s", p.Name, val))
				argIdx = len(args)
			}
		} else if argIdx < len(args) {
			assignments = append(assignments, fmt.Sprintf("%s=%s", p.Name, args[argIdx]))
			argIdx++
		} else if p.Default == "" {
			return nil, fmt.Errorf("recipe '%s' requires argument '%s'", r.Name, p.Name)
		}
	}

	return assignments, nil
}

func printUsage() {
	fmt.Print(`jmake - run justfile recipes via make

Usage:
  jmake [flags] [recipe] [args...]

Flags:
  -l, --list       List available recipes
  -d, --dump       Print generated Makefile to stdout
  -f, --file PATH  Specify justfile path
  -n, --dry-run    Show make command without executing
  -h, --help       Show this help
  -v, --version    Show version
`)
}
