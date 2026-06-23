package harness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// ---------------------------------------------------------------------------
// Task 7.5 — unit tests cho enrichGuard.allow + fetchTool + ghi file an toàn.
// ---------------------------------------------------------------------------

// newTestGuard dựng guard với allowlist tường minh và max_pages cố định.
func newTestGuard(maxPages int, allowedHosts []string, seeds []EnrichSeed) *enrichGuard {
	return newEnrichGuard(EnrichCaps{
		MaxPages:     maxPages,
		AllowedHosts: allowedHosts,
	}, seeds)
}

// TestEnrichGuardAllowHostAllowlist kiểm tra host in/out allowlist.
// Validates: Requirements 5.3, 5.4 (Property 12)
func TestEnrichGuardAllowHostAllowlist(t *testing.T) {
	guard := newTestGuard(10, []string{"go.dev", "pkg.go.dev"}, nil)

	cases := []struct {
		name    string
		url     string
		wantOK  bool
	}{
		{"allowed host", "https://go.dev/doc/effective_go", true},
		{"allowed host with path", "https://pkg.go.dev/net/http", true},
		{"host not in allowlist", "https://evil.example.com/page", false},
		{"subdomain not allowlisted", "https://sub.go.dev/x", false},
		{"unsupported scheme", "ftp://go.dev/file", false},
		{"missing host", "https:///path", false},
		{"empty url", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := guard.allow(tc.url)
			if ok != tc.wantOK {
				t.Fatalf("allow(%q) = %v (reason %q), want %v", tc.url, ok, reason, tc.wantOK)
			}
			if !ok && reason == "" {
				t.Fatalf("rejection of %q should include a reason", tc.url)
			}
		})
	}
}

// TestEnrichGuardSeedHostsAllowed kiểm tra host của seed URL luôn được phép
// dù không xuất hiện trong allowed_hosts.
// Validates: Requirements 5.3 (Property 12)
func TestEnrichGuardSeedHostsAllowed(t *testing.T) {
	guard := newTestGuard(10, nil, []EnrichSeed{{URL: "https://seed.example.org/start"}})
	if ok, reason := guard.allow("https://seed.example.org/other"); !ok {
		t.Fatalf("seed host should be allowed, got reject: %s", reason)
	}
	if ok, _ := guard.allow("https://not-seed.example.org/"); ok {
		t.Fatal("non-seed host should be rejected when allowlist empty")
	}
}

// TestEnrichGuardPageBudget kiểm tra hard cap số trang: khi fetched đạt
// max_pages, allow phải từ chối bất kể host hợp lệ. allow() không tự tăng
// fetched — loop runEnrich tăng sau mỗi fetch, nên ở đây ta set thủ công.
// Validates: Requirements 5.2 (Property 11)
func TestEnrichGuardPageBudget(t *testing.T) {
	guard := newTestGuard(3, []string{"go.dev"}, nil)
	url := "https://go.dev/page"

	// Mô phỏng loop: dưới budget thì allow, đạt budget thì reject.
	for i := 0; i < 3; i++ {
		if ok, reason := guard.allow(url); !ok {
			t.Fatalf("fetch %d should be allowed (reason %q)", i, reason)
		}
		guard.fetched++ // loop runEnrich tăng fetched sau mỗi fetch
	}
	ok, reason := guard.allow(url)
	if ok {
		t.Fatal("allow should reject once fetched reaches max_pages")
	}
	if !strings.Contains(reason, "max_pages") {
		t.Fatalf("expected max_pages reason, got %q", reason)
	}
}

// TestEnrichGuardMaxPagesDefault kiểm tra hard cap luôn tồn tại kể cả khi
// caps.MaxPages <= 0 (fallback defaultMaxPages).
// Validates: Requirements 5.2 (Property 11)
func TestEnrichGuardMaxPagesDefault(t *testing.T) {
	guard := newTestGuard(0, []string{"go.dev"}, nil)
	if guard.maxPages != defaultMaxPages {
		t.Fatalf("expected default max pages %d, got %d", defaultMaxPages, guard.maxPages)
	}
	guard.fetched = defaultMaxPages
	if ok, _ := guard.allow("https://go.dev/x"); ok {
		t.Fatal("allow should reject at default budget cap")
	}
}

// TestFetchToolStripsHTML kiểm tra fetchTool trả về text đã strip HTML,
// bỏ qua script/style.
// Validates: fetchTool behaviour (Requirements 5.x supporting)
func TestFetchToolStripsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><style>.x{color:red}</style></head>` +
			`<body><h1>Hello</h1><script>secret()</script><p>World</p></body></html>`))
	}))
	defer srv.Close()

	text, err := fetchTool(context.Background(), srv.URL, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Fatalf("expected stripped text to contain headings/paragraphs, got %q", text)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "color:red") {
		t.Fatalf("script/style content should be removed, got %q", text)
	}
}

// TestFetchToolTimeout kiểm tra fetchTool tôn trọng timeout: server chậm hơn
// timeout phải trả lỗi.
// Validates: Requirements 5.5 (timeout → fail-open at loop level)
func TestFetchToolTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("too late"))
	}))
	defer srv.Close()

	_, err := fetchTool(context.Background(), srv.URL, 30*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestFetchToolBlocksCrossHostRedirect kiểm tra redirect ra host khác bị chặn.
// Validates: Requirements 5.4 (Property 12)
func TestFetchToolBlocksCrossHostRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other-host.invalid/elsewhere", http.StatusFound)
	}))
	defer srv.Close()

	_, err := fetchTool(context.Background(), srv.URL, time.Second)
	if err == nil {
		t.Fatal("expected blocked cross-host redirect error, got nil")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("expected redirect-related error, got %v", err)
	}
}

// TestFetchToolAllowsSameHostRedirect kiểm tra redirect cùng host vẫn được theo.
func TestFetchToolAllowsSameHostRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dest", http.StatusFound)
	})
	mux.HandleFunc("/dest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<p>arrived</p>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	text, err := fetchTool(context.Background(), srv.URL+"/start", time.Second)
	if err != nil {
		t.Fatalf("same-host redirect should succeed, got %v", err)
	}
	if !strings.Contains(text, "arrived") {
		t.Fatalf("expected redirected content, got %q", text)
	}
}

// TestFetchToolFailOpenBadURL kiểm tra fetchTool trả lỗi (không panic) cho URL
// hỏng / không kết nối được — đây là tín hiệu để runEnrich fail-open tiếp tục
// loop (mức loop được kiểm chứng ở runEnrich logic).
// Validates: Requirements 5.5 (Property 14)
func TestFetchToolFailOpenBadURL(t *testing.T) {
	if _, err := fetchTool(context.Background(), "http://%zz-invalid", time.Second); err == nil {
		t.Fatal("expected error for malformed URL")
	}
	// Cổng đóng trên loopback → lỗi kết nối, vẫn trả error chứ không panic.
	if _, err := fetchTool(context.Background(), "http://127.0.0.1:1/never", 200*time.Millisecond); err == nil {
		t.Fatal("expected connection error for closed port")
	}
}

// ---------------------------------------------------------------------------
// Ghi file giới hạn trong docs root (confineToDocsRoot / writeReferenceDoc).
// Validates: Requirements 6.3 (Property 13)
// ---------------------------------------------------------------------------

func newTestController(projectRoot string) *LoopController {
	return &LoopController{
		Engine:   &Engine{ProjectRoot: projectRoot, Reporter: noopReporter{}},
		Reporter: noopReporter{},
	}
}

// TestConfineToDocsRoot kiểm tra path-safety: đường dẫn trong docs root được
// chấp nhận, đường dẫn thoát ra ngoài bị từ chối.
// Validates: Requirements 6.3 (Property 13)
func TestConfineToDocsRoot(t *testing.T) {
	root := t.TempDir()
	lc := newTestController(root)
	docsRoot := filepath.Join(root, "docs")

	okCases := []string{
		"docs/references/foo.md",
		"docs/modules/bar.md",
		"docs/a/b/c.md",
	}
	for _, rel := range okCases {
		abs, err := lc.confineToDocsRoot(rel)
		if err != nil {
			t.Fatalf("confineToDocsRoot(%q) unexpected error: %v", rel, err)
		}
		if !strings.HasPrefix(abs, docsRoot+string(os.PathSeparator)) && abs != docsRoot {
			t.Fatalf("resolved path %q escaped docs root %q", abs, docsRoot)
		}
	}

	badCases := []string{
		"../outside.md",
		"docs/../../etc/passwd",
		"docs/../secret.md",
		"/etc/passwd",
		"",
	}
	for _, rel := range badCases {
		if _, err := lc.confineToDocsRoot(rel); err == nil {
			t.Fatalf("confineToDocsRoot(%q) should have been rejected", rel)
		}
	}
}

// TestWriteReferenceDocConfined kiểm tra writeReferenceDoc ghi file trong docs
// root với frontmatter chuẩn type: reference.
// Validates: Requirements 6.1, 6.3 (Property 13)
func TestWriteReferenceDocConfined(t *testing.T) {
	root := t.TempDir()
	lc := newTestController(root)
	docsRoot := filepath.Join(root, "docs")

	abs, err := lc.writeReferenceDoc("docs/references", docChange{
		Title:   "My Great Reference",
		Content: "# Body\n\nSome content.",
	})
	if err != nil {
		t.Fatalf("writeReferenceDoc error: %v", err)
	}
	if !strings.HasPrefix(abs, docsRoot+string(os.PathSeparator)) {
		t.Fatalf("written path %q outside docs root %q", abs, docsRoot)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read written doc: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "type: reference") {
		t.Fatalf("reference doc missing frontmatter type, got:\n%s", content)
	}
	if !strings.Contains(content, "Some content.") {
		t.Fatalf("reference doc missing body content, got:\n%s", content)
	}
}

// TestPatchExistingDocConfined kiểm tra patchExistingDoc chỉ sửa file đã tồn
// tại trong docs root và từ chối path traversal.
// Validates: Requirements 6.2, 6.3 (Property 13)
func TestPatchExistingDocConfined(t *testing.T) {
	root := t.TempDir()
	lc := newTestController(root)

	// File đã tồn tại trong docs root → patch thành công.
	existing := filepath.Join(root, "docs", "modules", "harness.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := lc.patchExistingDoc(docChange{Path: "docs/modules/harness.md", Content: "new content"}); err != nil {
		t.Fatalf("patchExistingDoc on existing file failed: %v", err)
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "new content" {
		t.Fatalf("expected patched content, got %q", string(data))
	}

	// File chưa tồn tại → từ chối.
	if _, err := lc.patchExistingDoc(docChange{Path: "docs/modules/missing.md", Content: "x"}); err == nil {
		t.Fatal("patchExistingDoc should reject non-existent file")
	}
	// Path traversal → từ chối.
	if _, err := lc.patchExistingDoc(docChange{Path: "../escape.md", Content: "x"}); err == nil {
		t.Fatal("patchExistingDoc should reject path traversal")
	}
}

// ---------------------------------------------------------------------------
// Task 7.6 — property tests (testing/quick) cho guard.
// Random danh sách URL/host + allowlist → guard không bao giờ vượt max_pages
// hoặc cho qua host ngoài allowlist.
// Validates: Requirements 5.2 (Property 11), 5.3 (Property 12)
// ---------------------------------------------------------------------------

// hostPool là tập host cố định; allowMask bit chọn host nào vào allowlist.
var hostPool = []string{
	"go.dev", "pkg.go.dev", "example.com", "localhost",
	"docs.rs", "foo.bar", "a.io", "b.net",
}

// TestGuardPropertyNeverViolatesCaps là property test: với mọi allowlist,
// host, số fetched và max_pages ngẫu nhiên, nếu guard.allow trả true thì host
// PHẢI nằm trong allowlist VÀ fetched < max_pages; ngược lại khi fetched đã đạt
// max_pages thì allow PHẢI false.
func TestGuardPropertyNeverViolatesCaps(t *testing.T) {
	property := func(allowMask uint8, hostIdx uint8, fetched uint8, maxPagesByte uint8) bool {
		// Dựng allowlist từ bitmask.
		var allowedHosts []string
		for i, h := range hostPool {
			if allowMask&(1<<uint(i)) != 0 {
				allowedHosts = append(allowedHosts, h)
			}
		}
		// max_pages trong [1, 16] để hard cap luôn hữu hạn.
		maxPages := int(maxPagesByte%16) + 1
		guard := newTestGuard(maxPages, allowedHosts, nil)
		guard.fetched = int(fetched % 32) // có thể vượt hoặc dưới budget

		host := hostPool[int(hostIdx)%len(hostPool)]
		rawURL := "https://" + host + "/some/path"

		ok, _ := guard.allow(rawURL)

		// Property 11: đã đạt budget → không bao giờ allow.
		if guard.fetched >= guard.maxPages {
			return ok == false
		}
		// Property 12: nếu allow thì host phải nằm trong allowlist.
		if ok {
			return guard.allowed[normalizeHost(host)]
		}
		// Rejection luôn hợp lệ (an toàn theo hướng bảo thủ).
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatalf("guard cap property violated: %v", err)
	}
}

// TestGuardPropertyDisallowedHostAlwaysRejected là property test bổ sung tập
// trung Property 12: host KHÔNG nằm trong allowlist (và không phải seed) luôn
// bị từ chối, kể cả khi còn page budget.
func TestGuardPropertyDisallowedHostAlwaysRejected(t *testing.T) {
	property := func(allowMask uint8, hostIdx uint8) bool {
		var allowedHosts []string
		for i, h := range hostPool {
			if allowMask&(1<<uint(i)) != 0 {
				allowedHosts = append(allowedHosts, h)
			}
		}
		guard := newTestGuard(100, allowedHosts, nil) // budget dư dả
		host := hostPool[int(hostIdx)%len(hostPool)]
		rawURL := "https://" + host + "/x"

		ok, _ := guard.allow(rawURL)
		inAllowlist := guard.allowed[normalizeHost(host)]
		// allow chỉ được true khi host nằm trong allowlist.
		return ok == inAllowlist
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatalf("host allowlist property violated: %v", err)
	}
}

// TestConfineToDocsRootPropertyNeverEscapes là property test cho Property 13:
// với mọi chuỗi path ngẫu nhiên, confineToDocsRoot hoặc trả lỗi, hoặc trả về
// đường dẫn nằm trong docs root — không bao giờ thoát ra ngoài.
func TestConfineToDocsRootPropertyNeverEscapes(t *testing.T) {
	root := t.TempDir()
	lc := newTestController(root)
	docsRoot := lc.docsRoot()

	property := func(rel string) bool {
		abs, err := lc.confineToDocsRoot(rel)
		if err != nil {
			return true // từ chối là an toàn
		}
		// Đường dẫn được chấp nhận phải nằm trong docs root.
		relToDocs, rerr := filepath.Rel(docsRoot, abs)
		if rerr != nil {
			return false
		}
		return relToDocs != ".." && !strings.HasPrefix(relToDocs, ".."+string(os.PathSeparator))
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatalf("docs root confinement property violated: %v", err)
	}
}
