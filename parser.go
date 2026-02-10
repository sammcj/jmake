package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Param represents a recipe parameter.
type Param struct {
	Name     string
	Default  string // empty if required
	Variadic string // "" | "*" | "+"
}

// Variable represents a top-level variable assignment.
type Variable struct {
	Name     string
	Value    string
	Export   bool
	Backtick bool // value is a backtick command
}

// Alias maps one name to another recipe.
type Alias struct {
	Name   string
	Target string
}

// Recipe represents a justfile recipe.
type Recipe struct {
	Name         string
	Doc          string // doc comment (line immediately before recipe header)
	Params       []Param
	Dependencies []string
	Lines        []string // body lines (indented commands)
	Silent       bool     // all lines prefixed with @
}

// Justfile is the parsed representation of a justfile.
type Justfile struct {
	Variables []Variable
	Recipes   []Recipe
	Aliases   []Alias
}

var (
	// Section separator: lines like "# --- Section ---"
	sectionSepRe = regexp.MustCompile(`^#\s*---.*---\s*$`)

	// Variable assignment: name := "value" or name := `cmd`
	varAssignRe = regexp.MustCompile(`^(export\s+)?([a-zA-Z_][a-zA-Z0-9_-]*)\s*:=\s*(.+)$`)

	// Alias: alias name := target
	aliasRe = regexp.MustCompile(`^alias\s+([a-zA-Z_][a-zA-Z0-9_-]*)\s*:=\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*$`)

	// Recipe header: name param1 param2: dep1 dep2
	recipeHeaderRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)(\s+[^:]+)?:\s*(.*)$`)

	// Parameter patterns
	variadicParamRe = regexp.MustCompile(`^([*+])([a-zA-Z_][a-zA-Z0-9_-]*)$`)
	defaultParamRe  = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)=(.+)$`)
)

// Parse reads a justfile from r and returns a structured Justfile.
func Parse(r io.Reader) (*Justfile, error) {
	scanner := bufio.NewScanner(r)
	jf := &Justfile{}

	var (
		currentRecipe *Recipe
		pendingDoc    string
		lineNum       int
	)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// If we're inside a recipe and the line is indented, it's a body line.
		if currentRecipe != nil && len(line) > 0 && (line[0] == '\t' || strings.HasPrefix(line, "    ")) {
			// Strip one level of indentation (tab or 4 spaces).
			body := line
			if line[0] == '\t' {
				body = line[1:]
			} else if strings.HasPrefix(line, "    ") {
				body = line[4:]
			}
			currentRecipe.Lines = append(currentRecipe.Lines, body)
			continue
		}

		// Non-indented line ends current recipe.
		if currentRecipe != nil {
			jf.Recipes = append(jf.Recipes, *currentRecipe)
			currentRecipe = nil
		}

		trimmed := strings.TrimSpace(line)

		// Blank line resets pending doc.
		if trimmed == "" {
			pendingDoc = ""
			continue
		}

		// Section separators are not doc comments.
		if sectionSepRe.MatchString(trimmed) {
			pendingDoc = ""
			continue
		}

		// Comment line (potential doc comment).
		if after, ok := strings.CutPrefix(trimmed, "#"); ok {
			pendingDoc = strings.TrimSpace(after)
			continue
		}

		// Alias.
		if m := aliasRe.FindStringSubmatch(trimmed); m != nil {
			jf.Aliases = append(jf.Aliases, Alias{Name: m[1], Target: m[2]})
			pendingDoc = ""
			continue
		}

		// Variable assignment.
		if m := varAssignRe.FindStringSubmatch(trimmed); m != nil {
			isExport := strings.TrimSpace(m[1]) == "export"
			name := m[2]
			rawValue := strings.TrimSpace(m[3])

			v := Variable{Name: name, Export: isExport}

			if strings.HasPrefix(rawValue, "`") && strings.HasSuffix(rawValue, "`") {
				v.Value = rawValue[1 : len(rawValue)-1]
				v.Backtick = true
			} else {
				v.Value = unquote(rawValue)
			}

			jf.Variables = append(jf.Variables, v)
			pendingDoc = ""
			continue
		}

		// Recipe header.
		if m := recipeHeaderRe.FindStringSubmatch(trimmed); m != nil {
			recipe := Recipe{
				Name: m[1],
				Doc:  pendingDoc,
			}

			// Parse parameters from group 2.
			if paramStr := strings.TrimSpace(m[2]); paramStr != "" {
				recipe.Params = parseParams(paramStr)
			}

			// Parse dependencies from group 3.
			if depStr := strings.TrimSpace(m[3]); depStr != "" {
				recipe.Dependencies = parseDeps(depStr)
			}

			currentRecipe = &recipe
			pendingDoc = ""
			continue
		}

		// If nothing matched, reset pending doc.
		pendingDoc = ""
	}

	// Flush last recipe.
	if currentRecipe != nil {
		jf.Recipes = append(jf.Recipes, *currentRecipe)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading justfile: %w", err)
	}

	return jf, nil
}

// parseParams splits the parameter portion of a recipe header into Param values.
func parseParams(s string) []Param {
	var params []Param
	for tok := range strings.FieldsSeq(s) {
		p := Param{}

		if m := variadicParamRe.FindStringSubmatch(tok); m != nil {
			p.Variadic = m[1]
			p.Name = m[2]
		} else if m := defaultParamRe.FindStringSubmatch(tok); m != nil {
			p.Name = m[1]
			p.Default = unquote(m[2])
		} else {
			p.Name = tok
		}

		params = append(params, p)
	}
	return params
}

// parseDeps splits the dependency portion of a recipe header.
func parseDeps(s string) []string {
	var deps []string
	for d := range strings.FieldsSeq(s) {
		deps = append(deps, d)
	}
	return deps
}

// unquote strips surrounding quotes (single or double) from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
