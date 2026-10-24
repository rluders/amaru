package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Problem is one documentation defect, reported as path:line: message.
type Problem struct {
	Path    string // slash-separated, relative to the checked root
	Line    int    // 1-based; 0 when the problem concerns the whole file
	Message string
}

func (p Problem) String() string {
	if p.Line == 0 {
		return fmt.Sprintf("%s: %s", p.Path, p.Message)
	}
	return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message)
}

// Layout names the repository locations the structural checks look at.
// Paths are relative to the checked root.
type Layout struct {
	ADRDir        string // numbered ADR files, NNNN-title.md
	DecisionsFile string // ADR index that must list every ADR with its status
	SkillsDir     string // canonical Skills, <name>/SKILL.md
	ClaudeSkills  string // per-tool adapters: symlinks into SkillsDir
}

// DefaultLayout is the Amaru repository layout (docs/architecture/DECISIONS.md).
var DefaultLayout = Layout{
	ADRDir:        "docs/architecture/adr",
	DecisionsFile: "docs/architecture/DECISIONS.md",
	SkillsDir:     ".agents/skills",
	ClaudeSkills:  ".claude/skills",
}

// skippedDirs are never scanned for Markdown. testdata holds deliberately broken fixtures.
var skippedDirs = map[string]bool{".git": true, "node_modules": true, "testdata": true, "vendor": true}

// ADRStatuses is the closed set of ADR statuses (docs/architecture/DECISIONS.md).
var ADRStatuses = map[string]bool{
	"proposed": true, "under-investigation": true, "accepted": true,
	"rejected": true, "superseded": true, "deprecated": true,
}

// Check runs every check under root and returns the problems sorted by path and line.
func Check(root string, layout Layout) ([]Problem, error) {
	files, err := markdownFiles(root)
	if err != nil {
		return nil, err
	}
	docs := make(map[string]*document, len(files))
	for _, rel := range files {
		d, err := readDocument(root, rel)
		if err != nil {
			return nil, err
		}
		docs[rel] = d
	}

	var problems []Problem
	for _, rel := range files {
		problems = append(problems, docs[rel].fenceProblems...)
		problems = append(problems, checkLinks(root, docs[rel], docs)...)
	}
	problems = append(problems, checkADRs(root, layout, docs)...)
	problems = append(problems, checkSkills(root, layout)...)

	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Path != problems[j].Path {
			return problems[i].Path < problems[j].Path
		}
		return problems[i].Line < problems[j].Line
	})
	return problems, nil
}

func markdownFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Symlinked Skill directories are checked by checkSkills; WalkDir does not follow them.
		if strings.EqualFold(filepath.Ext(p), ".md") && d.Type().IsRegular() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// link is a Markdown inline link or image target found outside code.
type link struct {
	line   int
	target string
}

type document struct {
	path          string
	frontmatter   map[string]string
	anchors       map[string]bool
	links         []link
	fenceProblems []Problem
}

var (
	// [text](target) and ![alt](target); the target ends at whitespace or ')'.
	linkPattern       = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	inlineCodePattern = regexp.MustCompile("`[^`]*`")
	headingPattern    = regexp.MustCompile(`^#{1,6}\s+(.*?)\s*#*\s*$`)
	fencePattern      = regexp.MustCompile("^\\s{0,3}(```+|~~~+)\\s*([A-Za-z0-9_+-]*)")
)

func readDocument(root, rel string) (*document, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	d := &document{path: rel, anchors: map[string]bool{}}
	lines := splitLines(data)

	start := 0
	if fm, end, ok := parseFrontmatter(lines); ok {
		d.frontmatter = fm
		start = end
	}

	slugCount := map[string]int{}
	var fence string // opening fence marker while inside a fenced block
	fenceLang, fenceLine, fenceBody := "", 0, 0
	for i := start; i < len(lines); i++ {
		line, n := lines[i], i+1
		if m := fencePattern.FindStringSubmatch(line); m != nil {
			marker := m[1]
			if fence == "" {
				fence, fenceLang, fenceLine, fenceBody = marker, m[2], n, 0
				continue
			}
			if marker[0] == fence[0] && len(marker) >= len(fence) && strings.TrimSpace(line) == marker {
				if fenceLang == "mermaid" && fenceBody == 0 {
					d.fenceProblems = append(d.fenceProblems, Problem{rel, fenceLine, "empty mermaid block"})
				}
				fence = ""
				continue
			}
		}
		if fence != "" {
			if strings.TrimSpace(line) != "" {
				fenceBody++
			}
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			slug := slugify(m[1])
			if c := slugCount[slug]; c > 0 {
				d.anchors[fmt.Sprintf("%s-%d", slug, c)] = true
			} else {
				d.anchors[slug] = true
			}
			slugCount[slug]++
		}
		for _, m := range linkPattern.FindAllStringSubmatch(inlineCodePattern.ReplaceAllString(line, ""), -1) {
			d.links = append(d.links, link{line: n, target: m[1]})
		}
	}
	if fence != "" {
		d.fenceProblems = append(d.fenceProblems, Problem{rel, fenceLine, "unclosed code fence"})
	}
	return d, nil
}

func splitLines(data []byte) []string {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines
}

// parseFrontmatter reads a leading "---" block of single-line "key: value" pairs.
// It returns the index of the first line after the closing "---".
func parseFrontmatter(lines []string) (map[string]string, int, bool) {
	if len(lines) == 0 || lines[0] != "---" {
		return nil, 0, false
	}
	fm := map[string]string{}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return fm, i + 1, true
		}
		k, v, ok := strings.Cut(lines[i], ":")
		if !ok || strings.HasPrefix(lines[i], " ") {
			continue
		}
		fm[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return nil, 0, false
}

// slugify approximates GitHub's heading anchors: lowercase, drop punctuation
// other than '-' and '_', spaces become '-'. Inline Markdown is stripped first.
func slugify(heading string) string {
	heading = linkPattern.ReplaceAllStringFunc(heading, func(s string) string {
		return s[strings.Index(s, "[")+1 : strings.Index(s, "]")]
	})
	heading = strings.NewReplacer("`", "", "*", "").Replace(heading)
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func checkLinks(root string, d *document, docs map[string]*document) []Problem {
	var problems []Problem
	for _, l := range d.links {
		t := l.target
		if strings.Contains(t, "://") || strings.HasPrefix(t, "mailto:") {
			continue // external links are not checked offline
		}
		pathPart, anchor, _ := strings.Cut(t, "#")
		targetRel := d.path
		if pathPart != "" {
			if strings.HasPrefix(pathPart, "/") {
				problems = append(problems, Problem{d.path, l.line, fmt.Sprintf("absolute link %q: use a relative path", t)})
				continue
			}
			targetRel = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(d.path), pathPart)))
			if targetRel == ".." || strings.HasPrefix(targetRel, "../") {
				problems = append(problems, Problem{d.path, l.line, fmt.Sprintf("link %q leaves the repository", t)})
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(targetRel))); err != nil {
				problems = append(problems, Problem{d.path, l.line, fmt.Sprintf("broken link %q", t)})
				continue
			}
		}
		if anchor == "" {
			continue
		}
		target, ok := docs[targetRel]
		if !ok {
			continue // anchors are only checked inside scanned Markdown files
		}
		if !target.anchors[strings.ToLower(anchor)] {
			problems = append(problems, Problem{d.path, l.line, fmt.Sprintf("missing anchor %q in %s", "#"+anchor, targetRel)})
		}
	}
	return problems
}

var adrName = regexp.MustCompile(`^(\d{4})-[a-z0-9-]+\.md$`)
var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// checkADRs validates ADR file names and frontmatter, and that the index lists
// each ADR once with the same status as the file.
func checkADRs(root string, layout Layout, docs map[string]*document) []Problem {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(layout.ADRDir)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []Problem{{layout.ADRDir, 0, err.Error()}}
	}
	index, hasIndex := docs[layout.DecisionsFile]
	var indexLines []string
	if hasIndex {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(layout.DecisionsFile)))
		if err != nil {
			return []Problem{{layout.DecisionsFile, 0, err.Error()}}
		}
		indexLines = splitLines(data)
	}

	var problems []Problem
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		rel := layout.ADRDir + "/" + e.Name()
		m := adrName.FindStringSubmatch(e.Name())
		if m == nil {
			problems = append(problems, Problem{rel, 0, "ADR file name must be NNNN-lowercase-title.md"})
			continue
		}
		if m[1] == "0000" {
			continue // template
		}
		if other, dup := seen[m[1]]; dup {
			problems = append(problems, Problem{rel, 0, fmt.Sprintf("ADR number %s also used by %s", m[1], other)})
		}
		seen[m[1]] = e.Name()

		d := docs[rel]
		if d == nil || d.frontmatter == nil {
			problems = append(problems, Problem{rel, 1, "missing frontmatter"})
			continue
		}
		status := d.frontmatter["status"]
		if !ADRStatuses[status] {
			problems = append(problems, Problem{rel, 1, fmt.Sprintf("invalid status %q", status)})
		}
		if !isoDate.MatchString(d.frontmatter["date"]) {
			problems = append(problems, Problem{rel, 1, "date must be YYYY-MM-DD"})
		}
		if d.frontmatter["title"] == "" {
			problems = append(problems, Problem{rel, 1, "missing title"})
		}
		if status == "accepted" && d.frontmatter["deciders"] == "" {
			problems = append(problems, Problem{rel, 1, "accepted ADR must name its deciders"})
		}
		if !hasIndex {
			continue
		}
		problems = append(problems, checkIndexRow(layout, index.path, indexLines, e.Name(), status)...)
	}
	return problems
}

func checkIndexRow(layout Layout, indexPath string, lines []string, name, status string) []Problem {
	rel, err := filepath.Rel(filepath.Dir(layout.DecisionsFile), layout.ADRDir+"/"+name)
	if err != nil {
		return []Problem{{indexPath, 0, err.Error()}}
	}
	needle := "(" + filepath.ToSlash(rel) + ")"
	var rows []int
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "|") && strings.Contains(l, needle) {
			rows = append(rows, i)
		}
	}
	switch len(rows) {
	case 0:
		return []Problem{{indexPath, 0, fmt.Sprintf("ADR %s is not listed in the index table", name)}}
	case 1:
		cells := strings.Split(lines[rows[0]], "|")
		for _, c := range cells {
			if strings.TrimSpace(strings.Trim(c, " `*")) == status {
				return nil
			}
		}
		return []Problem{{indexPath, rows[0] + 1, fmt.Sprintf("index row for %s does not show status %q", name, status)}}
	default:
		return []Problem{{indexPath, rows[1] + 1, fmt.Sprintf("ADR %s is listed more than once", name)}}
	}
}

// checkSkills enforces one canonical copy per Skill: SkillsDir/<name>/SKILL.md with
// matching frontmatter, and ClaudeSkills/<name> as a symlink to that directory.
func checkSkills(root string, layout Layout) []Problem {
	canonical := filepath.Join(root, filepath.FromSlash(layout.SkillsDir))
	entries, err := os.ReadDir(canonical)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []Problem{{layout.SkillsDir, 0, err.Error()}}
	}
	var problems []Problem
	names := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		names[name] = true
		rel := layout.SkillsDir + "/" + name + "/SKILL.md"
		data, err := os.ReadFile(filepath.Join(canonical, name, "SKILL.md"))
		if err != nil {
			problems = append(problems, Problem{rel, 0, "missing SKILL.md"})
			continue
		}
		fm, _, ok := parseFrontmatter(splitLines(data))
		switch {
		case !ok:
			problems = append(problems, Problem{rel, 1, "missing frontmatter"})
		case fm["name"] != name:
			problems = append(problems, Problem{rel, 1, fmt.Sprintf("frontmatter name %q must equal directory name %q", fm["name"], name)})
		case fm["description"] == "":
			problems = append(problems, Problem{rel, 1, "missing description"})
		}
		problems = append(problems, checkAdapter(root, layout, name)...)
	}

	adapters, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(layout.ClaudeSkills)))
	if err != nil && !os.IsNotExist(err) {
		return append(problems, Problem{layout.ClaudeSkills, 0, err.Error()})
	}
	for _, a := range adapters {
		if !names[a.Name()] {
			problems = append(problems, Problem{layout.ClaudeSkills + "/" + a.Name(), 0,
				fmt.Sprintf("no canonical Skill in %s; Skills are authored there only", layout.SkillsDir)})
		}
	}
	return problems
}

func checkAdapter(root string, layout Layout, name string) []Problem {
	rel := layout.ClaudeSkills + "/" + name
	p := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(p)
	if err != nil {
		return []Problem{{rel, 0, "missing adapter symlink"}}
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return []Problem{{rel, 0, "must be a symlink to the canonical Skill, not a copy"}}
	}
	got, err1 := filepath.EvalSymlinks(p)
	want, err2 := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(layout.SkillsDir), name))
	if err1 != nil || err2 != nil || got != want {
		return []Problem{{rel, 0, fmt.Sprintf("symlink must resolve to %s/%s", layout.SkillsDir, name)}}
	}
	return nil
}
