package preview

import (
	"reflect"
	"testing"
)

// metaWant captures the moduleMeta fields a frontmatter/## Meta test case
// expects. Only the fields relevant to a case need to be set; an empty value
// asserts the field is empty.
type metaWant struct {
	Title       string
	Status      string
	Version     string
	Compliance  string
	Priority    string
	Description string
	Type        string
	Tags        []string
	Timestamp   string
}

func assertMeta(t *testing.T, got moduleMeta, want metaWant) {
	t.Helper()
	if got.Title != want.Title {
		t.Errorf("Title = %q, want %q", got.Title, want.Title)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
	if got.Version != want.Version {
		t.Errorf("Version = %q, want %q", got.Version, want.Version)
	}
	if got.Compliance != want.Compliance {
		t.Errorf("Compliance = %q, want %q", got.Compliance, want.Compliance)
	}
	if got.Priority != want.Priority {
		t.Errorf("Priority = %q, want %q", got.Priority, want.Priority)
	}
	if got.Description != want.Description {
		t.Errorf("Description = %q, want %q", got.Description, want.Description)
	}
	if got.Type != want.Type {
		t.Errorf("Type = %q, want %q", got.Type, want.Type)
	}
	if got.Timestamp != want.Timestamp {
		t.Errorf("Timestamp = %q, want %q", got.Timestamp, want.Timestamp)
	}
	if len(got.Tags) == 0 && len(want.Tags) == 0 {
		return
	}
	if !reflect.DeepEqual(got.Tags, want.Tags) {
		t.Errorf("Tags = %#v, want %#v", got.Tags, want.Tags)
	}
}

// TestParseDocumentMeta is the central table-driven suite covering the matrix
// of metadata sources: only `## Meta`, only frontmatter, both, malformed
// frontmatter, scalar vs array tags, and unknown type.
func TestParseDocumentMeta(t *testing.T) {
	const rel = "docs/foo.md"

	tests := []struct {
		name string
		raw  string
		want metaWant
	}{
		{
			// Property 6: backward compatibility — only `## Meta` parses as before.
			name: "only ## Meta",
			raw: "# Foo Doc\n\n## Meta\n\n" +
				"- **Status**: active\n" +
				"- **Version**: v1.0\n" +
				"- **Compliance**: current-state\n" +
				"- **Description**: A legacy doc\n",
			want: metaWant{
				Title:       "Foo Doc",
				Status:      "active",
				Version:     "v1.0",
				Compliance:  "current-state",
				Description: "A legacy doc",
			},
		},
		{
			// Property 8 & 10: only frontmatter, array tags, known keys.
			name: "only frontmatter",
			raw: "---\n" +
				"type: module\n" +
				"description: A module doc\n" +
				"tags: [preview, docs]\n" +
				"timestamp: 2026-05-27\n" +
				"status: active\n" +
				"version: current\n" +
				"---\n\n# Foo Doc\n\nBody text.\n",
			want: metaWant{
				Title:       "Foo Doc",
				Status:      "active",
				Version:     "current",
				Description: "A module doc",
				Type:        "module",
				Tags:        []string{"preview", "docs"},
				Timestamp:   "2026-05-27",
			},
		},
		{
			// Property 7: both present — frontmatter wins on overlap (status),
			// empty frontmatter fields filled from `## Meta` (compliance, priority).
			name: "both frontmatter wins and fills empty from Meta",
			raw: "---\n" +
				"type: feature\n" +
				"status: draft\n" +
				"tags: [a]\n" +
				"---\n\n# Foo Doc\n\n## Meta\n\n" +
				"- **Status**: active\n" +
				"- **Compliance**: current-state\n" +
				"- **Priority**: P0\n",
			want: metaWant{
				Title:      "Foo Doc",
				Status:     "draft", // frontmatter wins over ## Meta "active"
				Compliance: "current-state",
				Priority:   "P0",
				Type:       "feature",
				Tags:       []string{"a"},
			},
		},
		{
			// Property 9: malformed YAML → fallback to `## Meta`, no panic.
			name: "malformed frontmatter falls back to ## Meta",
			raw: "---\n" +
				"type: module\n" +
				"tags: [unclosed, broken\n" +
				"  : : :\n" +
				"status\n" +
				"---\n\n# Foo Doc\n\n## Meta\n\n" +
				"- **Status**: active\n" +
				"- **Version**: v2.0\n",
			want: metaWant{
				Title:   "Foo Doc",
				Status:  "active",
				Version: "v2.0",
				// Type from frontmatter is discarded because the whole block failed.
			},
		},
		{
			// Property 10: tags as a single scalar string → []string.
			name: "tags scalar string",
			raw: "---\n" +
				"type: reference\n" +
				"tags: preview\n" +
				"---\n\n# Foo Doc\n",
			want: metaWant{
				Title: "Foo Doc",
				Type:  "reference",
				Tags:  []string{"preview"},
			},
		},
		{
			// Property 10: tags as a sequence → []string.
			name: "tags array",
			raw: "---\n" +
				"tags:\n  - alpha\n  - beta\n  - gamma\n" +
				"---\n\n# Foo Doc\n",
			want: metaWant{
				Title: "Foo Doc",
				Tags:  []string{"alpha", "beta", "gamma"},
			},
		},
		{
			// Property 8: unknown type is kept verbatim (no enum validation),
			// and unknown keys are ignored without error.
			name: "unknown type and unknown keys are permissive",
			raw: "---\n" +
				"type: galaxy-brain\n" +
				"description: weird doc\n" +
				"unknown_key: some value\n" +
				"another: 42\n" +
				"tags: [x]\n" +
				"---\n\n# Foo Doc\n",
			want: metaWant{
				Title:       "Foo Doc",
				Description: "weird doc",
				Type:        "galaxy-brain",
				Tags:        []string{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDocumentMeta(rel, tt.raw)
			if got.Path != rel {
				t.Errorf("Path = %q, want %q", got.Path, rel)
			}
			assertMeta(t, got, tt.want)
		})
	}
}

// TestParseFrontmatterOK verifies the lower-level parseFrontmatter contract:
// presence detection (ok flag) and error reporting on malformed YAML.
func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantOK  bool
		wantErr bool
		check   func(t *testing.T, m moduleMeta)
	}{
		{
			name:   "no frontmatter returns ok=false no error",
			raw:    "# Foo\n\n## Meta\n\n- **Status**: active\n",
			wantOK: false,
		},
		{
			name:   "valid frontmatter returns ok=true no error",
			raw:    "---\ntype: module\nstatus: active\ntags: [a, b]\n---\n# Foo\n",
			wantOK: true,
			check: func(t *testing.T, m moduleMeta) {
				if m.Type != "module" {
					t.Errorf("Type = %q, want %q", m.Type, "module")
				}
				if m.Status != "active" {
					t.Errorf("Status = %q, want %q", m.Status, "active")
				}
				if !reflect.DeepEqual(m.Tags, []string{"a", "b"}) {
					t.Errorf("Tags = %#v, want [a b]", m.Tags)
				}
			},
		},
		{
			name:    "malformed frontmatter returns ok=true with error",
			raw:     "---\ntags: [unclosed, broken\n : : :\nstatus\n---\n# Foo\n",
			wantOK:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok, err := parseFrontmatter(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (err=%v)", ok, tt.wantOK, err)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil && err == nil {
				tt.check(t, meta)
			}
		})
	}
}

// TestNormalizeTags exercises tag normalization directly across scalar,
// sequence, empty, and whitespace shapes (Property 10).
func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name string
		in   frontmatterTags
		want []string
	}{
		{name: "nil yields nil", in: nil, want: nil},
		{name: "empty yields nil", in: frontmatterTags{}, want: nil},
		{name: "single scalar", in: frontmatterTags{"preview"}, want: []string{"preview"}},
		{name: "multiple", in: frontmatterTags{"a", "b", "c"}, want: []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTags(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeTags(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// TestFrontmatterTagsUnmarshalYAML verifies the scalar-vs-sequence YAML
// decoding behavior that backs tag normalization (Property 10) and the
// permissive handling of unexpected shapes (Property 8).
func TestFrontmatterTagsUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{name: "scalar string", yaml: "tags: preview", want: []string{"preview"}},
		{name: "sequence", yaml: "tags: [a, b]", want: []string{"a", "b"}},
		{name: "empty scalar", yaml: "tags: \"\"", want: nil},
		{name: "mapping shape ignored", yaml: "tags:\n  k: v", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := "---\n" + tt.yaml + "\n---\n# Foo\n"
			meta, ok, err := parseFrontmatter(raw)
			if !ok || err != nil {
				t.Fatalf("parseFrontmatter ok=%v err=%v", ok, err)
			}
			got := meta.Tags
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tags = %#v, want %#v", got, tt.want)
			}
		})
	}
}
