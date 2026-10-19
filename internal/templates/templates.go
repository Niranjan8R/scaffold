// Package templates provides the built-in templates (embedded in the binary)
// and loading of user-supplied template directories.
package templates

import (
	"embed"
	"io/fs"
	"os"
	"sort"

	"github.com/Niranjan8R/scaffold/internal/engine"
)

//go:embed all:builtin
var builtinFS embed.FS

func loadAll(fsys fs.FS, root string, skipMissingManifest bool) ([]*engine.Template, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	var out []*engine.Template
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := e.Name()
		if root != "." {
			dir = root + "/" + e.Name()
		}
		if skipMissingManifest {
			if _, err := fs.Stat(fsys, dir+"/"+engine.ManifestFile); err != nil {
				continue
			}
		}
		t, err := engine.Load(fsys, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Builtin returns the templates shipped inside the binary.
func Builtin() ([]*engine.Template, error) {
	return loadAll(builtinFS, "builtin", false)
}

// All returns built-in templates plus those in extraDir (if non-empty).
// A user template with the same name replaces the built-in one.
func All(extraDir string) ([]*engine.Template, error) {
	list, err := Builtin()
	if err != nil {
		return nil, err
	}
	if extraDir != "" {
		extra, err := loadAll(os.DirFS(extraDir), ".", true)
		if err != nil {
			return nil, err
		}
		byName := map[string]*engine.Template{}
		for _, t := range list {
			byName[t.Manifest.Name] = t
		}
		for _, t := range extra {
			byName[t.Manifest.Name] = t
		}
		list = list[:0]
		for _, t := range byName {
			list = append(list, t)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Manifest.Name < list[j].Manifest.Name })
	return list, nil
}

// Find returns the template called name, or nil.
func Find(list []*engine.Template, name string) *engine.Template {
	for _, t := range list {
		if t.Manifest.Name == name {
			return t
		}
	}
	return nil
}
