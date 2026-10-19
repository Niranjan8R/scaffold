// Command scaffold generates ready-to-build C++/CMake, CUDA and embedded
// (AVR) project skeletons, and can load your own templates from a directory.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Niranjan8R/scaffold/internal/engine"
	"github.com/Niranjan8R/scaffold/internal/templates"
)

var version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
		usage(errw)
		return 2
	}
	var err error
	switch args[0] {
	case "new":
		err = cmdNew(args[1:], in, out, errw)
	case "list":
		err = cmdList(args[1:], out, errw)
	case "version", "--version", "-v":
		fmt.Fprintln(out, "scaffold", version)
	case "help", "-h", "--help":
		usage(out)
	default:
		fmt.Fprintf(errw, "unknown command %q\n\n", args[0])
		usage(errw)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `scaffold - generate project skeletons

Usage:
  scaffold list [--template-dir DIR]
  scaffold new [flags] [template] [name]
  scaffold version

Flags for "new":
  --dir DIR            parent directory for the new project (default ".")
  --template-dir DIR   extra directory of custom templates
  --var key=value      set a template variable (repeatable)
  --yes                accept defaults, never prompt
  --force              overwrite existing files

Examples:
  scaffold new                              # fully interactive
  scaffold new cpp-cmake my-lib --var Tests=catch2
  scaffold new cuda vecadd --var Arch=86 --yes
  scaffold new embedded-avr blinky --dir ~/firmware
`)
}

// varFlags collects repeated --var key=value flags.
type varFlags struct{ m map[string]string }

func (v *varFlags) String() string { return "" }

func (v *varFlags) Set(s string) error {
	k, val, ok := strings.Cut(s, "=")
	if !ok || k == "" {
		return fmt.Errorf("expected key=value, got %q", s)
	}
	if v.m == nil {
		v.m = map[string]string{}
	}
	v.m[k] = val
	return nil
}

// parseInterspersed lets flags appear before, between or after positional args.
func parseInterspersed(flags *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		args = flags.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func cmdList(args []string, out, errw io.Writer) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(errw)
	tdir := flags.String("template-dir", "", "extra directory of custom templates")
	if _, err := parseInterspersed(flags, args); err != nil {
		return err
	}
	all, err := templates.All(*tdir)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, t := range all {
		fmt.Fprintf(tw, "%s\t%s\n", t.Manifest.Name, t.Manifest.Description)
	}
	return tw.Flush()
}

type prompter struct {
	r *bufio.Reader
	w io.Writer
}

func (p *prompter) ask(question, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(p.w, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(p.w, "%s: ", question)
	}
	line, err := p.r.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		if err == io.EOF && def != "" {
			return def, nil
		}
		return "", errors.New("input ended before all questions were answered")
	}
	if line == "" {
		return def, nil
	}
	return line, nil
}

func pickTemplate(p *prompter, all []*engine.Template) (*engine.Template, error) {
	fmt.Fprintln(p.w, "Available templates:")
	for i, t := range all {
		fmt.Fprintf(p.w, "  %d) %-14s %s\n", i+1, t.Manifest.Name, t.Manifest.Description)
	}
	ans, err := p.ask("Choose a template (number or name)", "1")
	if err != nil {
		return nil, err
	}
	if n, convErr := strconv.Atoi(ans); convErr == nil {
		if n < 1 || n > len(all) {
			return nil, fmt.Errorf("no template number %d", n)
		}
		return all[n-1], nil
	}
	if t := templates.Find(all, ans); t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("unknown template %q", ans)
}

func cmdNew(args []string, in io.Reader, out, errw io.Writer) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(errw)
	var userVars varFlags
	parent := flags.String("dir", ".", "parent directory for the new project")
	tdir := flags.String("template-dir", "", "extra directory of custom templates")
	yes := flags.Bool("yes", false, "accept defaults, never prompt")
	force := flags.Bool("force", false, "overwrite existing files")
	flags.Var(&userVars, "var", "template variable key=value (repeatable)")

	pos, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}
	if len(pos) > 2 {
		return fmt.Errorf("too many arguments: %v", pos[2:])
	}

	all, err := templates.All(*tdir)
	if err != nil {
		return err
	}
	p := &prompter{r: bufio.NewReader(in), w: out}

	var tmpl *engine.Template
	if len(pos) >= 1 {
		if tmpl = templates.Find(all, pos[0]); tmpl == nil {
			return fmt.Errorf("unknown template %q (run `scaffold list`)", pos[0])
		}
	} else {
		if *yes {
			return errors.New("a template name is required with --yes")
		}
		if tmpl, err = pickTemplate(p, all); err != nil {
			return err
		}
	}

	var name string
	if len(pos) == 2 {
		name = pos[1]
	} else {
		if *yes {
			return errors.New("a project name is required with --yes")
		}
		if name, err = p.ask("Project name", ""); err != nil {
			return err
		}
	}
	if err := engine.ValidateName(name); err != nil {
		return err
	}

	given := map[string]string{}
	for k, v := range userVars.m {
		given[k] = v
	}
	for k, v := range engine.Derive(name) { // derived values always win
		given[k] = v
	}
	if !*yes {
		for _, v := range tmpl.Manifest.Vars {
			if _, ok := given[v.Name]; ok {
				continue
			}
			label := v.Prompt
			if label == "" {
				label = v.Name
			}
			if len(v.Choices) > 0 {
				label += " (" + strings.Join(v.Choices, "/") + ")"
			}
			ans, err := p.ask(label, v.Default)
			if err != nil {
				return err
			}
			given[v.Name] = ans
		}
	}
	vars, err := tmpl.ResolveVars(given)
	if err != nil {
		return err
	}

	dest := filepath.Join(*parent, name)
	files, err := tmpl.Render(dest, vars, *force)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nCreated %s from template %q:\n", dest, tmpl.Manifest.Name)
	for _, f := range files {
		fmt.Fprintf(out, "  + %s\n", f)
	}
	next, err := tmpl.Next(vars)
	if err != nil {
		return err
	}
	if next != "" {
		fmt.Fprintf(out, "\nNext steps:\n%s\n", next)
	}
	return nil
}
