package agentsync

import (
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestPresetSkillFrontmatterIsValid is the repo-level guard: every shipped
// SKILL.md must carry frontmatter that parses as a YAML mapping with a
// `name` matching its directory and a non-empty `description`. Kiro shows
// "Invalid SKILL.md frontmatter" and drops the skill otherwise.
func TestPresetSkillFrontmatterIsValid(t *testing.T) {
	root := os.DirFS("../..")
	entries, err := fs.ReadDir(root, "presets/skills")
	if err != nil {
		t.Fatalf("read presets/skills: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := path.Join("presets/skills", entry.Name(), "SKILL.md")
		data, err := fs.ReadFile(root, rel)
		if err != nil {
			// Support dirs such as _shared carry no SKILL.md.
			continue
		}
		checked++
		_, body, _, ok := splitFrontmatter(string(data))
		if !ok {
			t.Errorf("%s: missing YAML frontmatter block", rel)
			continue
		}
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal([]byte(body), &meta); err != nil {
			t.Errorf("%s: frontmatter does not parse: %v\nhint: quote values containing \": \" (e.g. \"Trigger: ...\")", rel, err)
			continue
		}
		if meta.Name != entry.Name() {
			t.Errorf("%s: name = %q, want %q (must match directory)", rel, meta.Name, entry.Name())
		}
		if strings.TrimSpace(meta.Description) == "" {
			t.Errorf("%s: description is empty", rel)
		}
	}
	if checked == 0 {
		t.Fatal("no preset SKILL.md files were checked")
	}
}

func TestNormalizeSkillFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantChanged bool
		wantBody    string
	}{
		{
			name:        "unquoted description with Trigger colon",
			in:          "---\nname: fix\ndescription: Sửa bug. Trigger: fix lỗi, debug.\n---\n\n# Fix\n",
			wantChanged: true,
			wantBody:    "description: \"Sửa bug. Trigger: fix lỗi, debug.\"",
		},
		{
			name:        "already quoted stays byte-identical",
			in:          "---\nname: fix\ndescription: \"Sửa bug. Trigger: fix lỗi.\"\n---\n\n# Fix\n",
			wantChanged: false,
		},
		{
			name:        "plain valid frontmatter untouched",
			in:          "---\nname: gsap\ndescription: GSAP animation reference.\n---\n\n# GSAP\n",
			wantChanged: false,
		},
		{
			name:        "no frontmatter untouched",
			in:          "# Just a heading\n",
			wantChanged: false,
		},
		{
			name:        "unterminated frontmatter untouched",
			in:          "---\nname: broken\ndescription: x: y\n",
			wantChanged: false,
		},
		{
			name:        "irreparable frontmatter left as-is",
			in:          "---\ntags: [unclosed, broken\n  : : :\n---\n# x\n",
			wantChanged: false,
		},
		{
			name:        "nested mapping key not quoted",
			in:          "---\nname: x\nmeta:\n  owner: me\ndescription: A. Trigger: b, c.\n---\n# x\n",
			wantChanged: true,
			wantBody:    "description: \"A. Trigger: b, c.\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := normalizeSkillFrontmatter([]byte(tt.in))
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v (out=%q)", changed, tt.wantChanged, got)
			}
			if !tt.wantChanged {
				if string(got) != tt.in {
					t.Fatalf("unchanged case rewrote content:\n%q", got)
				}
				return
			}
			if !strings.Contains(string(got), tt.wantBody) {
				t.Fatalf("missing %q in:\n%s", tt.wantBody, got)
			}
			_, body, _, ok := splitFrontmatter(string(got))
			if !ok || !yamlParsesAsMapping(body) {
				t.Fatalf("repaired frontmatter still invalid:\n%s", got)
			}
			// Markdown body after the fence must survive untouched.
			if idx := strings.Index(tt.in, "---\n\n"); idx >= 0 {
				tail := tt.in[strings.LastIndex(tt.in, "---\n")+4:]
				if !strings.HasSuffix(string(got), tail) {
					t.Fatalf("body changed: want suffix %q, got:\n%s", tail, got)
				}
			}
		})
	}
}

func TestNeedsQuoting(t *testing.T) {
	quote := []string{"a: b", "trailing:", "text # comment", "Sửa bug. Trigger: x"}
	keep := []string{"", "plain text", `"already"`, "'single'", "[a, b]", "{a: 1}", "|", ">", "&anchor", "*ref", "!tag", "no-colon-here"}
	for _, v := range quote {
		if !needsQuoting(v) {
			t.Errorf("needsQuoting(%q) = false, want true", v)
		}
	}
	for _, v := range keep {
		if needsQuoting(v) {
			t.Errorf("needsQuoting(%q) = true, want false", v)
		}
	}
}

// TestInstallPresetTreeNormalizesUserSkillFrontmatter covers the overlay
// path: a user-supplied skill outside the repo also gets repaired, since
// repo linting cannot reach it.
func TestInstallPresetTreeNormalizesUserSkillFrontmatter(t *testing.T) {
	broken := "---\nname: custom-only\ndescription: Do things. Trigger: do, act.\n---\n\n# Custom\n"
	dir := t.TempDir()
	src := path.Join(dir, "custom.md")
	if err := os.WriteFile(src, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newTestContextWithOverlay(t, map[string]string{
		"presets/skills/custom-only/SKILL.md": src,
	})
	dstRoot := path.Join(t.TempDir(), "skills")
	op := InstallPresetTree{SrcRoot: "presets/skills", DstRoot: dstRoot, Replace: true}
	if err := op.Apply(ctx); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := os.ReadFile(path.Join(dstRoot, "custom-only", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, _, ok := splitFrontmatter(string(out))
	if !ok || !yamlParsesAsMapping(body) {
		t.Fatalf("user skill frontmatter not repaired:\n%s", out)
	}
}
