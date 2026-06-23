package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Hard cap defaults — code-enforced để không bao giờ crawl không giới hạn,
// kể cả khi task config bỏ trống (Requirements 5.2, 5.6).
const (
	defaultMaxPages     = 10
	defaultFetchTimeout = 15 * time.Second
	maxRedirects        = 10
	maxFetchBodyBytes   = 5 << 20 // 5 MiB an toàn cho việc strip HTML
)

// fetchedPage là một trang đã fetch trong một lần chạy enrichment.
// Dùng bởi runEnrich (task 7.3) để dựng corpus cho LLM tổng hợp.
type fetchedPage struct {
	URL  string
	Text string
}

// enrichGuard enforce hard caps; trạng thái mutable trong một lần chạy.
// Mọi giới hạn (host allowlist, page budget, timeout) là code-enforced,
// KHÔNG dựa vào LLM tự giới hạn (Requirements 5.6).
type enrichGuard struct {
	caps     EnrichCaps
	fetched  int
	allowed  map[string]bool // host set (lowercase, không port)
	maxPages int
}

// newEnrichGuard dựng guard với host allowlist = allowed_hosts ∪ host của seeds.
// Seeds được truyền vào để host của các seed URL luôn được phép fetch
// (Requirements 5.3). max_pages <= 0 fallback về defaultMaxPages để hard cap
// luôn tồn tại.
func newEnrichGuard(caps EnrichCaps, seeds []EnrichSeed) *enrichGuard {
	allowed := make(map[string]bool)
	for _, host := range caps.AllowedHosts {
		if h := normalizeHost(host); h != "" {
			allowed[h] = true
		}
	}
	for _, seed := range seeds {
		if seed.URL == "" {
			continue
		}
		if h := hostFromURL(seed.URL); h != "" {
			allowed[h] = true
		}
	}
	maxPages := caps.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}
	return &enrichGuard{
		caps:     caps,
		allowed:  allowed,
		maxPages: maxPages,
	}
}

// allow kiểm tra một URL có được fetch không (page budget + host). Code-enforced.
// Trả về reason rõ ràng khi từ chối để caller ghi warning.
func (g *enrichGuard) allow(rawURL string) (ok bool, reason string) {
	// 1. Page budget — hard cap số trang (Property 11, Requirements 5.2).
	if g.fetched >= g.maxPages {
		return false, fmt.Sprintf("max_pages reached (%d)", g.maxPages)
	}

	// 2. Parse + validate URL.
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false, fmt.Sprintf("invalid url: %v", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false, fmt.Sprintf("unsupported scheme %q", parsed.Scheme)
	}

	// 3. Host allowlist (Property 12, Requirements 5.3/5.4).
	host := normalizeHost(parsed.Hostname())
	if host == "" {
		return false, "missing host"
	}
	if !g.allowed[host] {
		return false, fmt.Sprintf("host %q not in allowlist", host)
	}
	return true, ""
}

// allowedHost trả về true nếu host (đã normalize) nằm trong allowlist.
// Dùng bởi fetchTool's CheckRedirect khi guard có sẵn.
func (g *enrichGuard) allowedHost(host string) bool {
	return g.allowed[normalizeHost(host)]
}

// timeout trả về timeout mỗi fetch từ caps, fallback default khi không cấu hình.
func (g *enrichGuard) timeout() time.Duration {
	if g.caps.FetchTimeoutSeconds > 0 {
		return time.Duration(g.caps.FetchTimeoutSeconds) * time.Second
	}
	return defaultFetchTimeout
}

// fetchTool fetch một URL với timeout, trả text đã strip HTML.
// CheckRedirect chặn redirect ra host khác host gốc — vì host gốc đã được
// guard.allow xác thực nằm trong allowlist, redirect cùng host sẽ luôn ở trong
// allowlist, còn redirect đổi host bị chặn (Property 12, Requirements 5.4).
func fetchTool(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	originHost := normalizeHost(parsed.Hostname())

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if normalizeHost(req.URL.Hostname()) != originHost {
				return fmt.Errorf("blocked redirect to host %q outside allowlist", req.URL.Hostname())
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ns-workspace-enrich/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: status %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read body %s: %w", rawURL, err)
	}

	return stripHTML(string(body)), nil
}

// stripHTML chuyển HTML thành plain text, bỏ qua script/style và collapse
// whitespace. Với nội dung không phải HTML, parser vẫn trả lại text gốc.
func stripHTML(raw string) string {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return strings.Join(strings.Fields(raw), " ")
	}
	var parts []string
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style":
				return
			}
		}
		if node.Type == html.TextNode {
			if text := strings.TrimSpace(node.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return strings.Join(parts, " ")
}

// normalizeHost chuẩn hoá host: lowercase, bỏ port.
func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		host = h
	}
	return strings.Trim(host, "[]")
}

// hostFromURL parse rawURL và trả về host đã normalize (rỗng nếu lỗi).
func hostFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return normalizeHost(parsed.Hostname())
}

// ---------------------------------------------------------------------------
// runEnrich loop (task 7.3): plan → fetch (guarded, fail-open) → execute →
// ghi file an toàn trong docs root. Guardrails là code-enforced; mọi side-effect
// ghi giới hạn trong docs root (Requirements 5.5, 6.1, 6.2, 6.3, 6.4).
// ---------------------------------------------------------------------------

const (
	// docsDirName là tên thư mục docs root mặc định dưới project root.
	docsDirName = "docs"
	// defaultReferencesDir là nơi tạo reference doc mới khi mode = references.
	defaultReferencesDir = "docs/references"
	// enrichCorpusCharLimit giới hạn độ dài text mỗi trang đưa vào execute prompt
	// để prompt không phình vô hạn.
	enrichCorpusCharLimit = 4000
)

// urlRegexp trích xuất URL http(s) thô từ text (plan output / fetched corpus).
var urlRegexp = regexp.MustCompile(`https?://[^\s)>\]"'` + "`" + `]+`)

// docChange là một thay đổi doc do execute phase (LLM) đề xuất.
// path: đường dẫn (tương đối project root); mode: references|enrich;
// content: nội dung markdown đầy đủ.
type docChange struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// urlDepth theo dõi độ sâu link-follow của một URL trong hàng đợi fetch.
type urlDepth struct {
	url   string
	depth int
}

// runEnrich điều phối enrichment khi task.Type == "enrich-docs".
// Loop.go (task 7.4) chịu trách nhiệm gọi hàm này; ở đây chỉ implement luồng.
func (lc *LoopController) runEnrich(ctx context.Context, task *Task, state *State) error {
	cfg := task.Enrich
	guard := newEnrichGuard(cfg.Caps, cfg.Seeds)
	rootCtx := WithProjectRoot(ctx, lc.Engine.ProjectRoot)

	// 1. PLAN: LLM đề xuất URL ứng viên từ seeds.
	planAgent := task.SelectAgent("plan")
	planRes, err := lc.Dispatcher.Resolve(planAgent).Dispatch(rootCtx, planAgent, buildEnrichPlanPrompt(cfg))
	if err != nil {
		return fmt.Errorf("enrich plan dispatch: %w", err)
	}
	if !planRes.Success {
		return fmt.Errorf("enrich plan subagent failed: %s", planRes.Stderr)
	}
	state.ContextNotes["last_plan"] = planRes.Stdout

	// Ứng viên = URL do LLM đề xuất ∪ seed URL (seed luôn được phép qua guard).
	var queue []urlDepth
	seen := map[string]bool{}
	enqueue := func(rawURL string, depth int) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || seen[rawURL] {
			return
		}
		seen[rawURL] = true
		queue = append(queue, urlDepth{url: rawURL, depth: depth})
	}
	for _, u := range parseURLList(planRes.Stdout) {
		enqueue(u, 0)
	}
	for _, seed := range cfg.Seeds {
		if seed.URL != "" {
			enqueue(seed.URL, 0)
		}
	}

	// 2. FETCH với guardrails code-enforced; fail-open khi lỗi.
	var corpus []fetchedPage
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		ok, reason := guard.allow(item.url)
		if !ok {
			lc.addEnrichWarning(state, fmt.Sprintf("skip %s: %s", item.url, reason))
			// Hết page budget thì dừng hẳn loop fetch.
			if guard.fetched >= guard.maxPages {
				break
			}
			continue
		}

		text, err := fetchTool(rootCtx, item.url, guard.timeout())
		guard.fetched++
		if err != nil {
			// fail-open: ghi warning, tiếp tục loop, KHÔNG dừng task (Req 5.5).
			lc.addEnrichWarning(state, fmt.Sprintf("fetch failed %s: %v", item.url, err))
			continue
		}
		corpus = append(corpus, fetchedPage{URL: item.url, Text: text})

		// Follow link chỉ khi còn depth budget; link mới vẫn qua guard.allow.
		if guard.caps.MaxDepth > 0 && item.depth < guard.caps.MaxDepth {
			for _, link := range parseURLList(text) {
				enqueue(link, item.depth+1)
			}
		}
	}

	// 3. EXECUTE: LLM tổng hợp corpus → đề xuất doc cần tạo/sửa.
	execAgent := task.SelectAgent("execute")
	execRes, err := lc.Dispatcher.Resolve(execAgent).Dispatch(rootCtx, execAgent, buildEnrichExecutePrompt(cfg, corpus))
	if err != nil {
		return fmt.Errorf("enrich execute dispatch: %w", err)
	}
	if !execRes.Success {
		return fmt.Errorf("enrich execute subagent failed: %s", execRes.Stderr)
	}
	state.ContextNotes["last_execute"] = execRes.Stdout

	changes := parseDocChanges(execRes.Stdout)

	// 4. WRITE: ghi file giới hạn trong docs root (Req 6.1/6.2/6.3).
	referencesDir := cfg.Target.ReferencesDir
	if strings.TrimSpace(referencesDir) == "" {
		referencesDir = defaultReferencesDir
	}
	var written []string
	for _, ch := range changes {
		var (
			path string
			werr error
		)
		if cfg.Target.Mode == "enrich" {
			path, werr = lc.patchExistingDoc(ch)
		} else {
			path, werr = lc.writeReferenceDoc(referencesDir, ch)
		}
		if werr != nil {
			lc.addEnrichWarning(state, fmt.Sprintf("write skipped %s: %v", ch.Path, werr))
			continue
		}
		written = append(written, path)
	}
	if len(written) > 0 {
		state.ContextNotes["enrich_written"] = strings.Join(written, "\n")
	}
	return nil
}

// addEnrichWarning ghi warning vào state và report ra ngoài (visibility).
func (lc *LoopController) addEnrichWarning(state *State, msg string) {
	state.AddWarning(msg)
	if lc.Reporter != nil {
		lc.Reporter.Line("enrich warning: %s", msg)
	}
}

// docsRoot trả về docs root tuyệt đối (đã clean) dưới project root.
func (lc *LoopController) docsRoot() string {
	return filepath.Clean(filepath.Join(lc.Engine.ProjectRoot, docsDirName))
}

// confineToDocsRoot resolve một đường dẫn (tương đối project root hoặc tuyệt đối)
// và từ chối nếu nó thoát khỏi docs root (path traversal, Req 6.3).
func (lc *LoopController) confineToDocsRoot(rel string) (string, error) {
	docsRoot := lc.docsRoot()
	target := strings.TrimSpace(rel)
	if target == "" {
		return "", fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(lc.Engine.ProjectRoot, target)
	}
	target = filepath.Clean(target)

	relToDocs, err := filepath.Rel(docsRoot, target)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", rel, err)
	}
	if relToDocs == ".." || strings.HasPrefix(relToDocs, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes docs root %q", rel, docsRoot)
	}
	return target, nil
}

// writeReferenceDoc tạo doc reference mới trong references_dir với frontmatter
// chuẩn `type: reference` (Req 6.1). Slug sinh từ title hoặc path của change.
func (lc *LoopController) writeReferenceDoc(referencesDir string, ch docChange) (string, error) {
	slug := slugify(referenceSlugSource(ch))
	if slug == "" {
		return "", fmt.Errorf("cannot derive slug for reference doc")
	}
	rel := filepath.Join(referencesDir, slug+".md")
	abs, err := lc.confineToDocsRoot(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	doc := buildReferenceFrontmatter(ch, slug) + stripFrontmatter(ch.Content)
	if err := os.WriteFile(abs, []byte(doc), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

// patchExistingDoc sửa một doc đã tồn tại bên trong docs root (Req 6.2).
// Từ chối nếu path thoát docs root hoặc file chưa tồn tại.
func (lc *LoopController) patchExistingDoc(ch docChange) (string, error) {
	if strings.TrimSpace(ch.Path) == "" {
		return "", fmt.Errorf("enrich change missing path")
	}
	abs, err := lc.confineToDocsRoot(ch.Path)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		return "", fmt.Errorf("enrich target %q is not an existing file", ch.Path)
	}
	if err := os.WriteFile(abs, []byte(ch.Content), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

// referenceSlugSource chọn nguồn để sinh slug: ưu tiên path basename, rồi title.
func referenceSlugSource(ch docChange) string {
	if p := strings.TrimSpace(ch.Path); p != "" {
		base := filepath.Base(p)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	return ch.Title
}

// slugify chuẩn hoá một chuỗi thành slug an toàn cho tên file.
func slugify(raw string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// buildReferenceFrontmatter dựng frontmatter chuẩn cho reference doc (type: reference).
func buildReferenceFrontmatter(ch docChange, slug string) string {
	title := strings.TrimSpace(ch.Title)
	if title == "" {
		title = slug
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: reference\n")
	b.WriteString(fmt.Sprintf("title: %s\n", title))
	b.WriteString(fmt.Sprintf("timestamp: %s\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("---\n\n")
	return b.String()
}

// stripFrontmatter loại bỏ block frontmatter `---...---` ở đầu content (nếu có)
// để buildReferenceFrontmatter là nguồn frontmatter duy nhất.
func stripFrontmatter(content string) string {
	trimmed := strings.TrimLeft(content, "\ufeff")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return content
	}
	// Tìm dấu đóng "---" trên một dòng riêng.
	rest := trimmed[strings.IndexByte(trimmed, '\n')+1:]
	lines := strings.Split(rest, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == "---" {
			return strings.TrimLeft(strings.Join(lines[i+1:], "\n"), "\n")
		}
	}
	// Không tìm thấy dấu đóng → trả nguyên content (permissive).
	return content
}

// parseURLList trích xuất danh sách URL http(s) từ output text, đã trim
// dấu câu cuối thường gặp trong markdown/prose.
func parseURLList(text string) []string {
	matches := urlRegexp.FindAllString(text, -1)
	var out []string
	for _, m := range matches {
		m = strings.TrimRight(m, ".,;:!?")
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}

// parseDocChanges parse JSON array các doc change từ execute output.
// Fail-open: trả nil nếu không tìm thấy/parse được JSON hợp lệ.
func parseDocChanges(output string) []docChange {
	blob := extractJSONArray(output)
	if blob == "" {
		return nil
	}
	var changes []docChange
	if err := json.Unmarshal([]byte(blob), &changes); err != nil {
		return nil
	}
	return changes
}

// extractJSONArray lấy chuỗi JSON array từ output: ưu tiên block ```json,
// fallback về đoạn từ '[' đầu tiên tới ']' cuối cùng.
func extractJSONArray(output string) string {
	if fenced := extractFencedJSON(output); fenced != "" {
		return fenced
	}
	start := strings.IndexByte(output, '[')
	end := strings.LastIndexByte(output, ']')
	if start >= 0 && end > start {
		return output[start : end+1]
	}
	return ""
}

// extractFencedJSON lấy nội dung trong block ```json ... ``` (nếu có).
func extractFencedJSON(output string) string {
	lower := strings.ToLower(output)
	idx := strings.Index(lower, "```json")
	if idx < 0 {
		return ""
	}
	rest := output[idx+len("```json"):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	}
	if close := strings.Index(rest, "```"); close >= 0 {
		return strings.TrimSpace(rest[:close])
	}
	return ""
}

// buildEnrichPlanPrompt dựng prompt cho plan phase: LLM đề xuất URL ứng viên
// trong giới hạn allowed_hosts + max_pages.
func buildEnrichPlanPrompt(cfg EnrichConfig) string {
	var b strings.Builder
	b.WriteString("You are the PLAN phase of a documentation enrichment task.\n")
	b.WriteString("Goal: propose candidate source URLs to fetch and summarize into docs.\n\n")
	b.WriteString("Seeds:\n")
	for _, seed := range cfg.Seeds {
		if seed.URL != "" {
			b.WriteString(fmt.Sprintf("- url: %s\n", seed.URL))
		}
		if seed.File != "" {
			b.WriteString(fmt.Sprintf("- file: %s\n", seed.File))
		}
	}
	maxPages := cfg.Caps.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}
	b.WriteString("\nHard caps (code-enforced, do not exceed):\n")
	b.WriteString(fmt.Sprintf("- max_pages: %d\n", maxPages))
	b.WriteString(fmt.Sprintf("- max_depth: %d\n", cfg.Caps.MaxDepth))
	if len(cfg.Caps.AllowedHosts) > 0 {
		b.WriteString(fmt.Sprintf("- allowed_hosts: %s\n", strings.Join(cfg.Caps.AllowedHosts, ", ")))
	} else {
		b.WriteString("- allowed_hosts: (seed hosts only)\n")
	}
	b.WriteString("\nReturn ONLY candidate URLs, one per line. URLs must be within the allowed hosts.\n")
	b.WriteString(fmt.Sprintf("Return at most %d URLs.\n", maxPages))
	return b.String()
}

// buildEnrichExecutePrompt dựng prompt cho execute phase: LLM tổng hợp corpus
// đã fetch thành các doc change dưới dạng JSON array.
func buildEnrichExecutePrompt(cfg EnrichConfig, corpus []fetchedPage) string {
	mode := cfg.Target.Mode
	if mode == "" {
		mode = "references"
	}
	var b strings.Builder
	b.WriteString("You are the EXECUTE phase of a documentation enrichment task.\n")
	b.WriteString(fmt.Sprintf("Target mode: %s\n", mode))
	if mode == "references" {
		b.WriteString("Create NEW reference docs summarizing the fetched content.\n")
	} else {
		b.WriteString("Patch EXISTING docs (paths must point to files already in docs/).\n")
	}
	b.WriteString("\nFetched corpus:\n")
	if len(corpus) == 0 {
		b.WriteString("(no pages were fetched)\n")
	}
	for i, page := range corpus {
		b.WriteString(fmt.Sprintf("\n--- Page %d: %s ---\n", i+1, page.URL))
		b.WriteString(truncateText(page.Text, enrichCorpusCharLimit))
		b.WriteString("\n")
	}
	b.WriteString("\nReturn the proposed document changes as a JSON array inside a ```json code block.\n")
	b.WriteString("Each item must have: {\"path\": string, \"mode\": \"" + mode + "\", \"title\": string, \"content\": string}.\n")
	b.WriteString("Paths MUST be relative to the project root and stay inside the docs/ directory.\n")
	b.WriteString("Content must be valid markdown.\n")
	return b.String()
}

// truncateText cắt text về tối đa limit ký tự, thêm marker khi bị cắt.
func truncateText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + " …(truncated)"
}
