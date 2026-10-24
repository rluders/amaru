package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree creates files (slash paths → content) under a fresh temp root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err) // e.g. Windows without developer mode
	}
}

func messages(problems []Problem) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = p.String()
	}
	return out
}

func assertProblems(t *testing.T, root string, want []string) {
	t.Helper()
	got, err := Check(root, DefaultLayout)
	if err != nil {
		t.Fatal(err)
	}
	g := messages(got)
	if strings.Join(g, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems:\n got: %q\nwant: %q", g, want)
	}
}

func TestLinks(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "valid relative links and anchors",
			files: map[string]string{
				"README.md":       "# Top\n[a](docs/a.md) [b](docs/a.md#second-part) [self](#top) [web](https://example.com/x)\n",
				"docs/a.md":       "# A\n## Second part\n[up](../README.md)\n![img](img.png)\n",
				"docs/img.png":    "",
				"docs/sub/dir.md": "[dir](..)\n",
			},
			want: nil,
		},
		{
			name: "broken file, anchor, absolute and escaping links",
			files: map[string]string{
				"README.md": "# Top\n[x](missing.md)\n[y](docs/a.md#nope)\n[z](/etc/passwd)\n[w](../outside.md)\n",
				"docs/a.md": "# A\n",
			},
			want: []string{
				`README.md:2: broken link "missing.md"`,
				`README.md:3: missing anchor "#nope" in docs/a.md`,
				`README.md:4: absolute link "/etc/passwd": use a relative path`,
				`README.md:5: link "../outside.md" leaves the repository`,
			},
		},
		{
			name: "links in code are ignored",
			files: map[string]string{
				"README.md": "`[x](missing.md)`\n```\n[y](missing.md)\n```\n",
			},
			want: nil,
		},
		{
			name: "duplicate headings get numbered anchors",
			files: map[string]string{
				"README.md": "# Notes\n## Notes\n[a](#notes-1) [b](#notes-2)\n",
			},
			want: []string{`README.md:3: missing anchor "#notes-2" in README.md`},
		},
		{
			name: "testdata directories are skipped",
			files: map[string]string{
				"tools/x/testdata/bad.md": "[x](missing.md)\n",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblems(t, writeTree(t, tt.files), tt.want)
		})
	}
}

func TestFences(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.md": "```mermaid\n\n```\n",
		"b.md": "text\n```go\nfunc f() {}\n",
		"c.md": "````md\n```\ninner\n```\n````\n```mermaid\nflowchart LR\n  A --> B\n```\n",
	})
	assertProblems(t, root, []string{
		"a.md:1: empty mermaid block",
		"b.md:2: unclosed code fence",
	})
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Hello World":                   "hello-world",
		"1. Product: scope & users":     "1-product-scope--users",
		"`code` and *emphasis*":         "code-and-emphasis",
		"[Linked](x.md) heading":        "linked-heading",
		"Cloud vs. self-hosted (draft)": "cloud-vs-self-hosted-draft",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

const adrOK = "---\ntitle: Use ADRs\nstatus: accepted\ndate: 2026-10-24\ndeciders: Someone\n---\n# 0001\n"

func TestADRs(t *testing.T) {
	index := func(rows string) string {
		return "# Decisions\n| ADR | Title | Status |\n|---|---|---|\n" + rows
	}
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "valid",
			files: map[string]string{
				"docs/architecture/DECISIONS.md":         index("| [0001](adr/0001-use-adrs.md) | Use ADRs | accepted |\n"),
				"docs/architecture/adr/0001-use-adrs.md": adrOK,
				"docs/architecture/adr/0000-template.md": "---\nstatus: TBD\n---\n",
				"docs/architecture/adr/README.md":        "# ADRs\n",
			},
		},
		{
			name: "status mismatch and missing row",
			files: map[string]string{
				"docs/architecture/DECISIONS.md":         index("| [0001](adr/0001-use-adrs.md) | Use ADRs | proposed |\n"),
				"docs/architecture/adr/0001-use-adrs.md": adrOK,
				"docs/architecture/adr/0002-other.md":    "---\ntitle: Other\nstatus: proposed\ndate: 2026-10-24\n---\n",
			},
			want: []string{
				"docs/architecture/DECISIONS.md: ADR 0002-other.md is not listed in the index table",
				`docs/architecture/DECISIONS.md:4: index row for 0001-use-adrs.md does not show status "accepted"`,
			},
		},
		{
			name: "bad frontmatter, name and duplicate number",
			files: map[string]string{
				"docs/architecture/adr/0001-a.md":   "---\nstatus: maybe\ndate: today\n---\n",
				"docs/architecture/adr/0001-b.md":   "---\ntitle: B\nstatus: accepted\ndate: 2026-10-24\n---\n",
				"docs/architecture/adr/0003-c.md":   "# no frontmatter\n",
				"docs/architecture/adr/Bad Name.md": "",
			},
			want: []string{
				"docs/architecture/adr/0001-a.md:1: invalid status \"maybe\"",
				"docs/architecture/adr/0001-a.md:1: date must be YYYY-MM-DD",
				"docs/architecture/adr/0001-a.md:1: missing title",
				"docs/architecture/adr/0001-b.md: ADR number 0001 also used by 0001-a.md",
				"docs/architecture/adr/0001-b.md:1: accepted ADR must name its deciders",
				"docs/architecture/adr/0003-c.md:1: missing frontmatter",
				"docs/architecture/adr/Bad Name.md: ADR file name must be NNNN-lowercase-title.md",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblems(t, writeTree(t, tt.files), tt.want)
		})
	}
}

func TestSkills(t *testing.T) {
	skill := func(name string) string {
		return "---\nname: " + name + "\ndescription: Does a thing.\n---\n# " + name + "\n"
	}

	t.Run("canonical skill with symlink adapter", func(t *testing.T) {
		root := writeTree(t, map[string]string{".agents/skills/review/SKILL.md": skill("review")})
		symlink(t, "../../.agents/skills/review", filepath.Join(root, ".claude/skills/review"))
		assertProblems(t, root, nil)
	})

	t.Run("copy, missing adapter, stray adapter, bad frontmatter", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			".agents/skills/copied/SKILL.md":   skill("copied"),
			".claude/skills/copied/SKILL.md":   skill("copied"),
			".agents/skills/lonely/SKILL.md":   skill("lonely"),
			".agents/skills/misnamed/SKILL.md": skill("other"),
			".agents/skills/empty/README.md":   "",
		})
		symlink(t, "../../.agents/skills/misnamed", filepath.Join(root, ".claude/skills/misnamed"))
		symlink(t, "../../.agents/skills/lonely/../copied", filepath.Join(root, ".claude/skills/stray"))
		assertProblems(t, root, []string{
			".agents/skills/empty/SKILL.md: missing SKILL.md",
			`.agents/skills/misnamed/SKILL.md:1: frontmatter name "other" must equal directory name "misnamed"`,
			".claude/skills/copied: must be a symlink to the canonical Skill, not a copy",
			".claude/skills/lonely: missing adapter symlink",
			".claude/skills/stray: no canonical Skill in .agents/skills; Skills are authored there only",
		})
	})

	t.Run("symlink to the wrong skill", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			".agents/skills/a/SKILL.md": skill("a"),
			".agents/skills/b/SKILL.md": skill("b"),
		})
		symlink(t, "../../.agents/skills/b", filepath.Join(root, ".claude/skills/a"))
		symlink(t, "../../.agents/skills/b", filepath.Join(root, ".claude/skills/b"))
		assertProblems(t, root, []string{".claude/skills/a: symlink must resolve to .agents/skills/a"})
	})
}
