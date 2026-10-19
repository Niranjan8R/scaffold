// Package engine loads project templates and renders them to disk.
//
// A template is a directory containing a template.json manifest plus any
// number of files. Rules:
//   - files ending in ".tmpl" are run through text/template, and the suffix is dropped
//   - all other files are copied byte-for-byte
//   - "__Var__" in a file or directory name is replaced by the value of Var
package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

const (
	ManifestFile = "template.json"
	TmplSuffix   = ".tmpl"
)

// Var describes one question a template asks the user.
type Var struct {
	Name    string   `json:"name"`
	Prompt  string   `json:"prompt"`
	Default string   `json:"default"`
	Choices []string `json:"choices,omitempty"`
}

// Manifest is the parsed template.json.
type Manifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Vars        []Var  `json:"vars"`
	Next        string `json:"next"` // optional "next steps" text, itself a template
}

// Template is a loaded template rooted at its own directory.
type Template struct {
	Manifest Manifest
	fsys     fs.FS
}

var nameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// ValidateName checks a project name is safe to use as a directory and identifier.
func ValidateName(n string) error {
	if !nameRe.MatchString(n) {
		return fmt.Errorf("invalid project name %q: use letters, digits, '-' or '_', starting with a letter (max 64)", n)
	}
	return nil
}

// Derive returns the variables computed from the project name.
func Derive(name string) map[string]string {
	ident := strings.ReplaceAll(name, "-", "_")
	return map[string]string{
		"Name":      name,
		"NameIdent": ident,
		"NameUpper": strings.ToUpper(ident),
	}
}

// Load reads the template stored in directory dir of fsys.
func Load(fsys fs.FS, dir string) (*Template, error) {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return nil, err
	}
	data, err := fs.ReadFile(sub, ManifestFile)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", dir, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("template %q: bad %s: %w", dir, ManifestFile, err)
	}
	if m.Name == "" {
		m.Name = path.Base(dir)
	}
	return &Template{Manifest: m, fsys: sub}, nil
}

// ResolveVars fills in defaults for missing variables and validates choices.
func (t *Template) ResolveVars(given map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(given)+len(t.Manifest.Vars))
	for k, v := range given {
		out[k] = v
	}
	for _, v := range t.Manifest.Vars {
		val := out[v.Name]
		if val == "" {
			val = v.Default
		}
		if len(v.Choices) > 0 && !slices.Contains(v.Choices, val) {
			return nil, fmt.Errorf("variable %s: %q is not one of %s", v.Name, val, strings.Join(v.Choices, ", "))
		}
		out[v.Name] = val
	}
	return out, nil
}

// Next renders the template's "next steps" message ("" if none).
func (t *Template) Next(vars map[string]string) (string, error) {
	if t.Manifest.Next == "" {
		return "", nil
	}
	b, err := renderText("next", []byte(t.Manifest.Next), vars)
	return string(b), err
}

func renderText(name string, src []byte, vars map[string]string) ([]byte, error) {
	tpl, err := template.New(name).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, vars); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func expandPath(p string, vars map[string]string) string {
	for k, v := range vars {
		p = strings.ReplaceAll(p, "__"+k+"__", v)
	}
	return p
}

func within(root, target string) bool {
	r, err := filepath.Rel(root, target)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

type outFile struct {
	rel    string
	target string
	data   []byte
	mode   fs.FileMode
}

// Render writes the template into dest and returns the relative paths created.
// Everything is rendered and checked in memory first, so a failure does not
// leave a half-written project behind.
func (t *Template) Render(dest string, vars map[string]string, force bool) ([]string, error) {
	var outs []outFile

	err := fs.WalkDir(t.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == ManifestFile {
			return nil
		}
		rel := expandPath(p, vars)
		isTmpl := strings.HasSuffix(rel, TmplSuffix)
		if isTmpl {
			rel = strings.TrimSuffix(rel, TmplSuffix)
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if !within(dest, target) {
			return fmt.Errorf("%s: path escapes the project directory", rel)
		}
		if !force {
			if _, statErr := os.Stat(target); statErr == nil {
				return fmt.Errorf("%s already exists (use --force to overwrite)", rel)
			}
		}
		data, err := fs.ReadFile(t.fsys, p)
		if err != nil {
			return err
		}
		if isTmpl {
			if data, err = renderText(p, data, vars); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		outs = append(outs, outFile{rel: rel, target: target, data: data, mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}

	created := make([]string, 0, len(outs))
	for _, o := range outs {
		if err := os.MkdirAll(filepath.Dir(o.target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(o.target, o.data, o.mode); err != nil {
			return nil, err
		}
		created = append(created, o.rel)
	}
	return created, nil
}
