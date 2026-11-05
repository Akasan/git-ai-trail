package commands

import "testing"

func TestCompileExcludes(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.md", "README.md", true},
		{"*.md", "docs/guide.md", true},
		{"*.md", "main.go", false},
		{"vendor/", "vendor/lib/a.go", true},
		{"vendor/", "pkg/vendor/a.go", true},
		{"vendor/", "vendor.go", false},
		{"testdata", "internal/testdata/x.json", true},
		{"docs/*.md", "docs/guide.md", true},
		{"docs/*.md", "docs/sub/guide.md", false},
		{"docs/*.md", "pkg/docs/guide.md", false},
		{"docs/**/*.md", "docs/sub/deep/guide.md", true},
		{"docs/**/*.md", "docs/guide.md", true},
		{"/main.go", "main.go", true},
		{"/main.go", "cmd/main.go", false},
		{"**/*_gen.go", "a/b/x_gen.go", true},
		{"gen/**", "gen/a/b.go", true},
		{"file?.go", "file1.go", true},
		{"file?.go", "file10.go", false},
		{"a.b", "axb", false},
		{"ドキュメント/", "ドキュメント/a.md", true},
	}

	for _, tt := range tests {
		res, err := compileExcludes([]string{tt.pattern})
		if err != nil {
			t.Fatalf("compileExcludes(%q) error: %v", tt.pattern, err)
		}
		if got := isExcluded(tt.path, res); got != tt.want {
			t.Errorf("pattern %q, path %q: got %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestCompileExcludesSkipsBlank(t *testing.T) {
	res, err := compileExcludes([]string{"", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expected no matchers, got %d", len(res))
	}
}
