package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestNewBuiltins(t *testing.T) {
	cases := map[string][]string{
		"cpp-cmake":    {"CMakeLists.txt", "src/main.cpp", "tests/CMakeLists.txt", ".gitignore"},
		"cuda":         {"CMakeLists.txt", "src/main.cu", "include/cuda_check.cuh"},
		"embedded-avr": {"Makefile", "src/main.c", "cmake/avr-gcc.cmake"},
	}
	for tpl, files := range cases {
		dir := t.TempDir()
		code, _, errs := runCLI(t, "", "new", tpl, "demo-app", "--dir", dir, "--yes")
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", tpl, code, errs)
		}
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(dir, "demo-app", f)); err != nil {
				t.Errorf("%s: missing %s", tpl, f)
			}
		}
		// no unrendered template actions may be left behind
		filepath.Walk(filepath.Join(dir, "demo-app"), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(p)
				if bytes.Contains(b, []byte("{{")) {
					t.Errorf("%s: unrendered action in %s", tpl, p)
				}
			}
			return nil
		})
	}
}

func TestCatch2Variant(t *testing.T) {
	dir := t.TempDir()
	code, _, errs := runCLI(t, "", "new", "cpp-cmake", "x", "--var", "Tests=catch2", "--dir", dir, "--yes")
	if code != 0 {
		t.Fatal(errs)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "x", "tests", "test_greeter.cpp"))
	if !strings.Contains(string(b), "catch2") {
		t.Fatalf("expected Catch2 test, got:\n%s", b)
	}
}

func TestInteractive(t *testing.T) {
	dir := t.TempDir()
	code, out, errs := runCLI(t, "cpp-cmake\nmyproj\n\n\n", "new", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "myproj", "CMakeLists.txt")); err != nil {
		t.Fatalf("project not created; output:\n%s", out)
	}
}

func TestErrors(t *testing.T) {
	if code, _, _ := runCLI(t, "", "new", "nope", "x", "--yes"); code != 1 {
		t.Error("unknown template should fail")
	}
	if code, _, _ := runCLI(t, "", "new", "cuda", "../bad", "--yes"); code != 1 {
		t.Error("bad name should fail")
	}
	dir := t.TempDir()
	runCLI(t, "", "new", "cuda", "dup", "--dir", dir, "--yes")
	if code, _, _ := runCLI(t, "", "new", "cuda", "dup", "--dir", dir, "--yes"); code != 1 {
		t.Error("second run should refuse to overwrite")
	}
	if code, _, _ := runCLI(t, "", "new", "cuda", "dup", "--dir", dir, "--yes", "--force"); code != 0 {
		t.Error("--force should overwrite")
	}
}

func TestCustomTemplateDir(t *testing.T) {
	dir := t.TempDir()
	code, out, errs := runCLI(t, "", "list", "--template-dir", "examples/templates")
	if code != 0 || !strings.Contains(out, "python-pkg") {
		t.Fatalf("custom template not listed: %s %s", out, errs)
	}
	code, _, errs = runCLI(t, "", "new", "python-pkg", "my-tool", "--template-dir", "examples/templates", "--dir", dir, "--yes")
	if code != 0 {
		t.Fatal(errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "my-tool", "src", "my_tool", "__init__.py")); err != nil {
		t.Fatal("__NameIdent__ path placeholder not expanded")
	}
}
