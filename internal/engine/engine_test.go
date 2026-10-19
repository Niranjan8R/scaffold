package engine

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"t/template.json": {Data: []byte(`{"name":"t","vars":[{"name":"Std","default":"17","choices":["14","17"]}],"next":"cd {{.Name}}"}`)},
		"t/src/__Name__.cpp.tmpl": {Data: []byte("// {{.Name}} std={{.Std}}\n")},
		"t/raw.txt":               {Data: []byte("{{.Name}}\n")},
	}
}

func TestRenderAndForce(t *testing.T) {
	tm, err := Load(testFS(), "t")
	if err != nil {
		t.Fatal(err)
	}
	vars, err := tm.ResolveVars(Derive("demo"))
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	files, err := tm.Render(dest, vars, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %v", files)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "src", "demo.cpp"))
	if string(got) != "// demo std=17\n" {
		t.Fatalf("unexpected render: %q", got)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "raw.txt"))
	if string(raw) != "{{.Name}}\n" {
		t.Fatalf("non-.tmpl files must be copied verbatim, got %q", raw)
	}
	if _, err := tm.Render(dest, vars, false); err == nil {
		t.Fatal("expected conflict error on second render")
	}
	if _, err := tm.Render(dest, vars, true); err != nil {
		t.Fatalf("force render failed: %v", err)
	}
	next, _ := tm.Next(vars)
	if next != "cd demo" {
		t.Fatalf("next = %q", next)
	}
}

func TestChoiceRejected(t *testing.T) {
	tm, _ := Load(testFS(), "t")
	given := Derive("demo")
	given["Std"] = "99"
	if _, err := tm.ResolveVars(given); err == nil {
		t.Fatal("expected invalid choice error")
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	tm, _ := Load(testFS(), "t")
	vars, _ := tm.ResolveVars(map[string]string{"Name": "../../evil"})
	if _, err := tm.Render(t.TempDir(), vars, false); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"a", "my-app", "My_App2"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1abc", "a b", "../x", "a/b", "-x"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
