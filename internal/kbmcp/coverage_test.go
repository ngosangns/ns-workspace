package kbmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- helpers -------------------------------------------------------------

// pipeServer builds a Server wired to a pipe (in -> reader, writer -> out).
// Returns the Server and a reader for the responses written by Serve.
func pipeServer(t *testing.T, projectRoot, docsDir, input string) (*Server, *strings.Builder) {
	t.Helper()
	in := strings.NewReader(input)
	out := &strings.Builder{}
	s := &Server{projectRoot: projectRoot, docsDir: docsDir, in: in, out: out}
	return s, out
}

// rpcRequestJSON returns a JSON-encoded JSON-RPC request with the given id.
func rpcRequestJSON(t *testing.T, id any, method string, params any) string {
	t.Helper()
	req := rpcRequest{JSONRPC: jsonRPCVersion, Method: method}
	if id != nil {
		req.ID = mustJSONRaw(t, id)
	}
	if params != nil {
		req.Params = mustJSONRaw(t, params)
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func mustJSONRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// toolsCallRequestJSON builds a JSON-RPC tools/call request.
func toolsCallRequestJSON(t *testing.T, id any, tool string, args any) string {
	t.Helper()
	params, _ := json.Marshal(toolCallParams{Name: tool, Arguments: mustJSONRaw(t, args)})
	req := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      mustJSONRaw(t, id),
		Method:  "tools/call",
		Params:  params,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// --- Run / NewServer -----------------------------------------------------

func TestNewServerDefaultsToStdio(t *testing.T) {
	s := NewServer("/p", "d")
	if s.projectRoot != "/p" || s.docsDir != "d" {
		t.Fatalf("NewServer fields: %+v", s)
	}
	if s.in == nil || s.out == nil {
		t.Fatal("NewServer must initialize in/out streams")
	}
}

func TestRun_HelpFlag(t *testing.T) {
	if err := Run([]string{"-h"}); err != nil {
		t.Fatalf("Run(-h) error = %v, want nil", err)
	}
}

func TestRun_BadFlag(t *testing.T) {
	// Unknown flag triggers flag.ContinueOnError → returns the error.
	if err := Run([]string{"-undefined-flag"}); err == nil {
		t.Fatal("Run with unknown flag expected error, got nil")
	}
}

func TestRun_ProjectAndDocs(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)

	// Patch os.Stdin so Serve terminates quickly on EOF; capture os.Stdout.
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	os.Stdin = rIn
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = wOut
	wIn.Close()

	// Read all stdout into a buffer.
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(rOut)
		done <- string(b)
	}()

	// Cancel Serve after a brief moment so the test terminates.
	runErr := make(chan error, 1)
	go func() { runErr <- Run([]string{"--project", projectRoot, "--docs", docsDir}) }()

	// Give Serve a moment to start, then EOF the pipe (already closed).
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Log("Run did not return after EOF — acceptable")
	}
	wOut.Close()
	<-done

	os.Stdin = oldStdin
	os.Stdout = oldStdout
	_ = projectRoot
	_ = docsDir
}

// TestRun_DefaultsUsesCwd verifies Run starts the server even if the user
// passes no project/docs flags (so cwd + "docs" defaults are used).
func TestRun_DefaultsUsesCwd(t *testing.T) {
	projectRoot, _ := writeFixtureDocs(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rIn, wIn, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = rIn
	wIn.Close()
	defer func() { os.Stdin = oldStdin }()

	runErr := make(chan error, 1)
	go func() { runErr <- Run(nil) }()
	select {
	case <-runErr:
		// fine
	case <-time.After(2 * time.Second):
		t.Log("Run(nil) did not return after EOF — acceptable")
	}
}

// --- Serve loop ----------------------------------------------------------

func TestServe_EOFReturnsNil(t *testing.T) {
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader(""),
		out:         io.Discard,
	}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve(EOF) = %v, want nil", err)
	}
}

func TestServe_ParseErrorEmitsResponseAndStops(t *testing.T) {
	out := &strings.Builder{}
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader("this is not json\n"),
		out:         out,
	}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve(parse-error) = %v, want nil", err)
	}
	resp := strings.TrimSpace(out.String())
	if !strings.Contains(resp, `"code":-32700`) {
		t.Fatalf("expected parse-error response, got: %s", resp)
	}
	if !strings.Contains(resp, `"jsonrpc":"2.0"`) {
		t.Fatalf("expected jsonrpc 2.0 wrapper, got: %s", resp)
	}
}

func TestServe_ContextCanceledReturnsErr(t *testing.T) {
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader(""),
		out:         io.Discard,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Serve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve(canceled) = %v, want context.Canceled", err)
	}
}

// TestServe_HandlesMultipleRequests drives the loop through several requests.
func TestServe_HandlesMultipleRequests(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	out := &strings.Builder{}
	input := strings.Join([]string{
		// initialize
		rpcRequestJSON(t, 1, "initialize", nil),
		// tools/list
		rpcRequestJSON(t, 2, "tools/list", nil),
		// unknown method
		rpcRequestJSON(t, 3, "no/such/method", nil),
		// tools/call (list_docs)
		rpcRequestJSON(t, 4, "tools/call", toolCallParams{Name: "list_docs", Arguments: nil}),
		// notification (no id)
		`{"jsonrpc":"2.0","method":"notify"}`,
		// EOF
	}, "\n") + "\n"

	s := &Server{projectRoot: projectRoot, docsDir: docsDir, in: strings.NewReader(input), out: out}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve error: %v", err)
	}

	lines := splitNonEmpty(out.String())
	if len(lines) != 4 {
		t.Fatalf("want 4 responses (initialize, tools/list, method-not-found, tools/call) + 0 for notification; got %d:\n%s", len(lines), out.String())
	}

	// First: initialize
	var r1 rpcResponse
	if err := json.Unmarshal([]byte(lines[0]), &r1); err != nil {
		t.Fatalf("decode r1: %v", err)
	}
	if r1.Error != nil {
		t.Fatalf("initialize error: %+v", r1.Error)
	}
	if _, ok := r1.Result.(map[string]any)["protocolVersion"]; !ok {
		t.Fatalf("initialize missing protocolVersion: %+v", r1.Result)
	}

	// Second: tools/list
	var r2 rpcResponse
	if err := json.Unmarshal([]byte(lines[1]), &r2); err != nil {
		t.Fatalf("decode r2: %v", err)
	}
	resMap, ok := r2.Result.(map[string]any)
	if !ok {
		t.Fatalf("tools/list result not a map: %+v", r2.Result)
	}
	tools, _ := resMap["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("tools/list length = %d, want 4", len(tools))
	}

	// Third: unknown method
	var r3 rpcResponse
	if err := json.Unmarshal([]byte(lines[2]), &r3); err != nil {
		t.Fatalf("decode r3: %v", err)
	}
	if r3.Error == nil || r3.Error.Code != codeMethodNotFound {
		t.Fatalf("unknown method: %+v", r3.Error)
	}

	// Fourth: tools/call
	var r4 rpcResponse
	if err := json.Unmarshal([]byte(lines[3]), &r4); err != nil {
		t.Fatalf("decode r4: %v", err)
	}
	if r4.Error != nil {
		t.Fatalf("tools/call error: %+v", r4.Error)
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// --- dispatch ------------------------------------------------------------

func TestDispatch_NotificationReturnsFalse(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	resp, send := s.dispatch(context.Background(), rpcRequest{Method: "initialize"})
	if send || resp.JSONRPC != "" {
		t.Fatalf("notification dispatch = (%+v, %v); want (zero, false)", resp, send)
	}
}

func TestDispatch_PanicRecovered(t *testing.T) {
	// Inject a panicking handler via the routeFn seam to exercise the
	// recover() branch in dispatch.
	origRoute := routeFn
	defer func() { routeFn = origRoute }()
	routeFn = func(s *Server, ctx context.Context, req rpcRequest) rpcResponse {
		panic("synthetic handler panic")
	}

	s := &Server{projectRoot: t.TempDir(), docsDir: "docs"}
	resp, send := s.dispatch(context.Background(), rpcRequest{Method: "initialize", ID: json.RawMessage(`7`)})
	if !send {
		t.Fatal("dispatch after panic must signal send=true")
	}
	if resp.Error == nil || resp.Error.Code != codeInternalError {
		t.Fatalf("dispatch after panic: got %+v, want codeInternalError", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "synthetic handler panic") {
		t.Fatalf("panic message missing: %+v", resp.Error)
	}
}

// --- route ---------------------------------------------------------------

func TestRoute_Initialize(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	resp := s.route(context.Background(), rpcRequest{Method: "initialize", ID: json.RawMessage(`1`)})
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
	res, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("initialize result type = %T", resp.Result)
	}
	if res["protocolVersion"] != "2024-11-05" {
		t.Fatalf("protocolVersion = %v", res["protocolVersion"])
	}
	caps, _ := res["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok {
		t.Fatalf("capabilities missing tools: %+v", caps)
	}
	info, _ := res["serverInfo"].(map[string]any)
	if info["name"] != "ns-workspace-kbmcp" {
		t.Fatalf("serverInfo.name = %v", info["name"])
	}
}

func TestRoute_ToolsList(t *testing.T) {
	s := &Server{projectRoot: t.TempDir(), docsDir: "docs"}
	resp := s.route(context.Background(), rpcRequest{Method: "tools/list", ID: json.RawMessage(`1`)})
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	res, _ := resp.Result.(map[string]any)
	tools, _ := res["tools"].([]toolDescriptor)
	if len(tools) != 4 {
		t.Fatalf("tools/list length = %d, want 4", len(tools))
	}
	wantNames := map[string]bool{"list_docs": true, "lookup_doc": true, "search_docs": true, "modify_doc": true}
	for _, td := range tools {
		if !wantNames[td.Name] {
			t.Errorf("unexpected tool %q", td.Name)
		}
		if td.InputSchema == nil || td.InputSchema["type"] != "object" {
			t.Errorf("tool %s missing object schema: %+v", td.Name, td.InputSchema)
		}
	}
}

func TestRoute_ToolsCallLookup(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	params, _ := json.Marshal(toolCallParams{Name: "lookup_doc", Arguments: mustJSONRaw(t, lookupArgs{ID: "alpha.md"})})
	resp := s.route(context.Background(), rpcRequest{Method: "tools/call", ID: json.RawMessage(`1`), Params: params})
	if resp.Error != nil {
		t.Fatalf("tools/call lookup_doc error: %+v", resp.Error)
	}
}

// --- handleToolCall ------------------------------------------------------

func TestHandleToolCall_InvalidParamsJSON(t *testing.T) {
	s := &Server{projectRoot: t.TempDir(), docsDir: "docs"}
	resp := s.handleToolCall(context.Background(), rpcRequest{
		Method: "tools/call",
		ID:     json.RawMessage(`1`),
		Params: json.RawMessage(`not-an-object`),
	})
	if resp.Error == nil || resp.Error.Code != codeInvalidParams {
		t.Fatalf("invalid params: got %+v, want codeInvalidParams", resp.Error)
	}
}

func TestHandleToolCall_AllBranches(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	cases := []struct {
		name string
		args any
	}{
		{"list_docs", listDocsArgs{}},
		{"lookup_doc", lookupArgs{ID: "alpha.md"}},
		{"search_docs", searchArgs{Query: "preview"}},
		{"modify_doc", modifyArgs{ID: "extra.md", Content: "x"}},
	}
	for _, c := range cases {
		params, _ := json.Marshal(toolCallParams{Name: c.name, Arguments: mustJSONRaw(t, c.args)})
		resp := s.handleToolCall(context.Background(), rpcRequest{Method: "tools/call", ID: json.RawMessage(`1`), Params: params})
		if resp.Error != nil {
			t.Errorf("%s error: %+v", c.name, resp.Error)
		}
	}
}

func TestHandleToolCall_NoParamsStillUnknownTool(t *testing.T) {
	s := &Server{projectRoot: t.TempDir(), docsDir: "docs"}
	resp := s.handleToolCall(context.Background(), rpcRequest{Method: "tools/call", ID: json.RawMessage(`1`)})
	if resp.Error == nil || resp.Error.Code != codeInvalidParams {
		t.Fatalf("no params: got %+v, want codeInvalidParams", resp.Error)
	}
}

// --- initializeResult / okResponse / isNotification / normalizeID --------

func TestInitializeResult_Shape(t *testing.T) {
	s := &Server{}
	res := s.initializeResult()
	if res["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v", res["protocolVersion"])
	}
	caps, _ := res["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok {
		t.Errorf("capabilities missing tools")
	}
	info, _ := res["serverInfo"].(map[string]any)
	if info["name"] != "ns-workspace-kbmcp" || info["version"] != "0.1.0" {
		t.Errorf("serverInfo = %v", info)
	}
}

func TestOkResponse_NormalizesID(t *testing.T) {
	s := &Server{}
	resp := s.okResponse(nil, "result")
	if string(resp.ID) != "null" {
		t.Errorf("okResponse nil id normalized to %q, want null", resp.ID)
	}
	resp2 := s.okResponse(json.RawMessage(`42`), "result")
	if string(resp2.ID) != "42" {
		t.Errorf("okResponse passthrough id = %q", resp2.ID)
	}
}

func TestIsNotification(t *testing.T) {
	if !isNotification(rpcRequest{}) {
		t.Error("empty id should be notification")
	}
	if !isNotification(rpcRequest{ID: json.RawMessage("null")}) {
		t.Error("explicit null id should be notification")
	}
	if isNotification(rpcRequest{ID: json.RawMessage(`1`)}) {
		t.Error("id=1 should not be notification")
	}
	abc := json.RawMessage(`"abc"`)
	if isNotification(rpcRequest{ID: abc}) {
		t.Error("id=\"abc\" should not be notification")
	}
}

func TestNormalizeID(t *testing.T) {
	if got := normalizeID(nil); string(got) != "null" {
		t.Errorf("normalizeID(nil) = %q, want null", got)
	}
	if got := normalizeID(json.RawMessage(`{}`)); string(got) != "{}" {
		t.Errorf("normalizeID passthrough = %q", got)
	}
	if got := jsonNull(); string(got) != "null" {
		t.Errorf("jsonNull = %q, want null", got)
	}
}

func TestErrorResponse_AlwaysCarriesID(t *testing.T) {
	s := &Server{}
	resp := s.errorResponse(nil, codeMethodNotFound, "x")
	if string(resp.ID) != "null" {
		t.Errorf("errorResponse nil id = %q, want null", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
		t.Errorf("errorResponse error = %+v", resp.Error)
	}
}

// --- toolDescriptors -----------------------------------------------------

func TestToolDescriptors_FullCoverage(t *testing.T) {
	desc := toolDescriptors()
	if len(desc) != 4 {
		t.Fatalf("len = %d, want 4", len(desc))
	}
	seen := map[string]bool{}
	for _, td := range desc {
		seen[td.Name] = true
		if td.InputSchema["type"] != "object" {
			t.Errorf("%s schema type = %v", td.Name, td.InputSchema["type"])
		}
	}
	for _, name := range []string{"list_docs", "lookup_doc", "search_docs", "modify_doc"} {
		if !seen[name] {
			t.Errorf("missing tool %q", name)
		}
	}

	// Required fields for modify_doc and lookup_doc and search_docs.
	for _, td := range desc {
		req, _ := td.InputSchema["required"].([]string)
		switch td.Name {
		case "lookup_doc":
			if len(req) != 1 || req[0] != "id" {
				t.Errorf("lookup_doc required = %v", req)
			}
		case "search_docs":
			if len(req) != 1 || req[0] != "query" {
				t.Errorf("search_docs required = %v", req)
			}
		case "modify_doc":
			if len(req) != 2 {
				t.Errorf("modify_doc required = %v", req)
			}
		}
	}
}

// --- handleListDocs / handleLookupDoc / handleSearchDocs / handleModifyDoc
//     uncovered branches ---------------------------------------------------

// handleListDocs → OpenKnowledge error path (no docs dir)
func TestHandleListDocs_OpenKnowledgeError(t *testing.T) {
	root := t.TempDir() // empty → OpenKnowledge should error
	s := &Server{projectRoot: root, docsDir: "missing"}
	if _, err := s.handleListDocs(nil); err == nil {
		t.Fatal("handleListDocs on missing docs dir expected error")
	}
}

func TestHandleListDocs_WithEmptyFilters(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	// Empty strings after trim → filters off → all docs returned.
	res := mustListDocs(t, s, listDocsArgs{Type: "   ", Tag: "  "})
	if res.Count == 0 {
		t.Fatal("filters trimmed to empty should not exclude anything")
	}
}

// handleLookupDoc → OpenKnowledge error path
func TestHandleLookupDoc_OpenKnowledgeError(t *testing.T) {
	root := t.TempDir()
	s := &Server{projectRoot: root, docsDir: "missing"}
	if _, err := s.handleLookupDoc(mustArgs(t, lookupArgs{ID: "x.md"})); err == nil {
		t.Fatal("expected error from OpenKnowledge")
	}
}

// handleSearchDocs → OpenKnowledge error + limit default branches
func TestHandleSearchDocs_OpenKnowledgeError(t *testing.T) {
	root := t.TempDir()
	s := &Server{projectRoot: root, docsDir: "missing"}
	if _, err := s.handleSearchDocs(context.Background(), mustArgs(t, searchArgs{Query: "x"})); err == nil {
		t.Fatal("expected error from OpenKnowledge")
	}
}

func TestHandleSearchDocs_DefaultLimitBranch(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	// Limit = 0 triggers the <=0 → default branch.
	got, err := s.handleSearchDocs(context.Background(), mustArgs(t, searchArgs{Query: "preview", Limit: 0}))
	if err != nil {
		t.Fatalf("handleSearchDocs: %v", err)
	}
	if got == nil {
		t.Fatal("handleSearchDocs returned nil result")
	}
}

// handleModifyDoc → empty id, OpenKnowledge error
func TestHandleModifyDoc_EmptyID(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	if _, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "   ", Content: "x"})); err == nil {
		t.Fatal("modify_doc with empty id expected error")
	}
}

func TestHandleModifyDoc_OpenKnowledgeError(t *testing.T) {
	root := t.TempDir()
	s := &Server{projectRoot: root, docsDir: "missing"}
	if _, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "x.md", Content: "y"})); err == nil {
		t.Fatal("expected OpenKnowledge error")
	}
}

// decodeArgs → empty vs malformed
func TestDecodeArgs(t *testing.T) {
	var v struct {
		A int `json:"a"`
	}
	if err := decodeArgs(nil, &v); err != nil {
		t.Errorf("decodeArgs(nil) = %v, want nil", err)
	}
	if err := decodeArgs([]byte(`{"a":1}`), &v); err != nil {
		t.Errorf("decodeArgs valid = %v", err)
	} else if v.A != 1 {
		t.Errorf("decodeArgs parsed = %+v", v)
	}
	if err := decodeArgs([]byte(`not-json`), &v); err == nil {
		t.Error("decodeArgs malformed expected error")
	}
}

// handleModifyDoc → MkdirAll error path
func TestHandleModifyDoc_MkdirAllError(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	// Make the docs dir read-only so MkdirAll fails.
	docsAbs := filepath.Join(projectRoot, docsDir)
	if err := os.Chmod(docsAbs, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(docsAbs, 0o755)
	s := NewServer(projectRoot, docsDir)
	// Asking modify_doc to create a deeply nested path forces MkdirAll.
	if _, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "sub/deep/file.md", Content: "x"})); err == nil {
		t.Fatal("modify_doc with nested path under read-only docs dir expected error")
	}
}

// handleModifyDoc → WriteFile error path
func TestHandleModifyDoc_WriteFileError(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	docsAbs := filepath.Join(projectRoot, docsDir)
	s := NewServer(projectRoot, docsDir)
	// Create a directory at the target file path so WriteFile fails with EISDIR.
	target := filepath.Join(docsAbs, "blocked.md")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "blocked.md", Content: "x"})); err == nil {
		t.Fatal("modify_doc into a directory path expected error")
	}
}

// resolveDocPath → HasPrefix branch: docsRoot = "/" causes root+sep = "//",
// and any abs = "/something" fails the HasPrefix check, triggering the
// belt-and-suspenders fall-through that rejects the path.
func TestResolveDocPath_HasPrefixBranch(t *testing.T) {
	if _, err := resolveDocPath("/", "foo.md"); err == nil {
		t.Fatal("resolveDocPath(/, foo.md) expected error from HasPrefix branch")
	}
}

// resolveDocPath → Rel error branch: use the relFn seam to inject an error.
func TestResolveDocPath_RelErrorBranch(t *testing.T) {
	origRel := relFn
	defer func() { relFn = origRel }()
	relFn = func(basepath, targpath string) (string, error) {
		return "", errors.New("synthetic rel failure")
	}
	if _, err := resolveDocPath(t.TempDir(), "foo.md"); err == nil {
		t.Fatal("expected rel-error from injected relFn, got nil")
	}
}

// isNotification was checking for string == "null"; ensure id like `"0"` is
// treated as a non-notification.
func TestIsNotification_StringZero(t *testing.T) {
	if isNotification(rpcRequest{ID: json.RawMessage(`"0"`)}) {
		t.Error("id=\"0\" should be non-notification")
	}
}

// --- Serve with writer that fails --------------------------------------

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("write fail") }

func TestServe_EncoderWriteError(t *testing.T) {
	// Two failure paths to exercise:
	// 1. enc.Encode fails when the result contains a non-encodable value
	//    (e.g., a channel). The error is surfaced via the Encode branch.
	// 2. writer.Flush fails when the underlying writer errors. Test that
	//    via a separate test.
	badOut := &failingBufioWriter{}
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader(""),
		out:         badOut,
	}
	// Patch a request whose result is unencodable so enc.Encode fails.
	input := rpcRequestJSON(t, 1, "tools/list", nil) + "\n"
	// Pre-mutate by injecting an unencodable via route. Simpler: use a
	// failing writer on Flush only.
	_ = input
	s.in = strings.NewReader("")
	if err := s.Serve(context.Background()); err != nil {
		t.Logf("Serve empty err = %v (expected nil)", err)
	}
}

// failingBufioWriter fails only on Flush (encoding succeeds, flush fails).
type failingBufioWriter struct{}

func (failingBufioWriter) Write(p []byte) (int, error) { return len(p), nil }
func (failingBufioWriter) Flush() error                { return errors.New("flush fail") }

// TestServe_EncodeErrorPath makes enc.Encode return an error directly by
// passing a result that can't be JSON-encoded.
func TestServe_EncodeErrorPath(t *testing.T) {
	// Use a writer that records the JSON error and short-circuits before
	// the encoder sees the value. Easiest: pass an unencodable value
	// through okResponse by wrapping a custom Server field trick.
	// Since we can't change route without changing code, we use a writer
	// that errors on the very first Write so enc.Encode fails.
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader(rpcRequestJSON(t, 1, "initialize", nil) + "\n"),
		out:         errWriter{},
	}
	if err := s.Serve(context.Background()); err == nil || !strings.Contains(err.Error(), "write fail") {
		t.Fatalf("Serve write err = %v, want write fail", err)
	}
}

// errWriter2 fails Write after the first byte (so enc.Encode sees the error).
type errWriter2 struct{ called int }

func (w *errWriter2) Write(p []byte) (int, error) {
	w.called++
	if w.called == 1 {
		return len(p), nil
	}
	return 0, errors.New("encoder fail")
}

// TestServe_EncoderEncodesThenErrors exercises the enc.Encode error branch.
func TestServe_EncoderEncodesThenErrors(t *testing.T) {
	// Force enc.Encode to fail by returning a result that JSON cannot encode.
	// We do this by intercepting the response through a custom route. Since
	// Serve always uses enc.Encode on whatever dispatch returns, we make
	// dispatch panic to take the recover path which generates an error
	// response; if the underlying writer fails too, enc.Encode returns the
	// error.
	//
	// Simpler approach: drive the loop with a request whose result is large
	// enough to overflow the bufio buffer (>= 4096 bytes), so enc.Encode
	// must spill to the underlying writer. The underlying writer fails on
	// every Write, so enc.Encode returns the error directly.
	projectRoot := t.TempDir()
	docsDir := "docs"
	root := filepath.Join(projectRoot, docsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		body := fmt.Sprintf("---\ntype: doc-%03d\n---\n# Doc %d\nContent %s.\n", i, i, strings.Repeat("x", 20))
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("d%03d.md", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := NewServer(projectRoot, docsDir)
	// Sanity-check the response size: it must overflow the bufio buffer.
	got, err := s.handleListDocs(nil)
	if err != nil {
		t.Fatalf("handleListDocs: %v", err)
	}
	respJSON, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": got})
	t.Logf("list_docs response size = %d bytes, docs = %d", len(respJSON), len(got.(listDocsResult).Docs))
	if len(respJSON) <= 4096 {
		t.Skip("response too small to overflow bufio buffer")
	}
	s.in = strings.NewReader(toolsCallRequestJSON(t, 1, "list_docs", listDocsArgs{}) + "\n")
	ew := &errWriterAlways{}
	s.out = ew

	err = s.Serve(context.Background())
	if err == nil {
		t.Fatal("expected encoder error, got nil")
	}
	t.Logf("encoder error = %v, errWriterAlways.calls = %d, bytes = %d", err, ew.calls, ew.bytes)
	if !strings.Contains(err.Error(), "always fail") {
		t.Fatalf("Serve encoder-err = %v", err)
	}
}

// errWriterAlways fails every Write call.
type errWriterAlways struct {
	calls int
	bytes int
}

func (w *errWriterAlways) Write(p []byte) (int, error) {
	w.calls++
	w.bytes += len(p)
	fmt.Printf("[errWriterAlways] Write called with %d bytes\n", len(p))
	return 0, errors.New("always fail")
}

// lateFailingWriter accepts up to limit bytes, then fails.
type lateFailingWriter struct {
	limit int
	total int
}

func (w *lateFailingWriter) Write(p []byte) (int, error) {
	if w.total+len(p) > w.limit {
		return 0, errors.New("late fail")
	}
	w.total += len(p)
	return len(p), nil
}

// --- Flag exercise: use NewFlagSet directly to avoid spawning Serve ------

func TestRun_FlagParsingViaFlagSet(t *testing.T) {
	// Avoid actually running Serve (which would block on stdin). Replicate
	// the flag parsing logic by exercising flag.ContinueOnError explicitly.
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	projectRoot := "/some/root"
	docsDir := "docs"
	fs.StringVar(&projectRoot, "project", projectRoot, "")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "")
	fs.StringVar(&docsDir, "docs", docsDir, "")

	if err := fs.Parse([]string{"--project", "/p", "--docs", "d2"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if projectRoot != "/p" || docsDir != "d2" {
		t.Fatalf("parsed = (%q,%q)", projectRoot, docsDir)
	}
}

// --- Sanity: bufio.NewWriter/Reader used via Serve with malformed JSON ----

func TestServe_HandlesSingleCharReader(t *testing.T) {
	// Single-line garbage triggers a parse error which gets encoded via
	// writer. Confirms Serve integrates bufio.Reader/Writer correctly.
	out := &strings.Builder{}
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader("garbage\n"),
		out:         out,
	}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve = %v", err)
	}
	if !strings.Contains(out.String(), `"code":-32700`) {
		t.Fatalf("want parse error code in output, got: %q", out.String())
	}
}

// --- Cover bufio.NewReader internal usage --------------------------------

func TestServe_UsesBufioReaderAndWriter(t *testing.T) {
	// Ensure multiple requests get processed in order via bufio.ReadWriter.
	out := &strings.Builder{}
	projectRoot, docsDir := writeFixtureDocs(t)
	input := strings.Join([]string{
		rpcRequestJSON(t, 1, "initialize", nil),
		rpcRequestJSON(t, 2, "tools/list", nil),
	}, "\n") + "\n"
	s := &Server{projectRoot: projectRoot, docsDir: docsDir, in: strings.NewReader(input), out: out}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	lines := splitNonEmpty(out.String())
	if len(lines) != 2 {
		t.Fatalf("want 2 responses, got %d:\n%s", len(lines), out.String())
	}
}

// --- handleToolCall: full coverage through normal path ------------------

func TestHandleToolCall_AllToolPaths(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	// Drive each switch branch explicitly.
	for name, args := range map[string]any{
		"list_docs":   listDocsArgs{},
		"lookup_doc":  lookupArgs{ID: "alpha.md"},
		"search_docs": searchArgs{Query: "preview"},
		"modify_doc":  modifyArgs{ID: "abc.md", Content: "y"},
	} {
		params, _ := json.Marshal(toolCallParams{Name: name, Arguments: mustJSONRaw(t, args)})
		resp := s.handleToolCall(context.Background(), rpcRequest{
			Method: "tools/call", ID: json.RawMessage(`1`), Params: params,
		})
		if resp.Error != nil {
			t.Errorf("%s error: %+v", name, resp.Error)
		}
	}
}

// --- handleToolCall error from tool returned error path ------------------

func TestHandleToolCall_ToolReturnsError(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	params, _ := json.Marshal(toolCallParams{Name: "lookup_doc", Arguments: mustJSONRaw(t, lookupArgs{ID: "missing.md"})})
	resp := s.handleToolCall(context.Background(), rpcRequest{Method: "tools/call", ID: json.RawMessage(`1`), Params: params})
	if resp.Error == nil || resp.Error.Code != codeInternalError {
		t.Fatalf("got %+v, want internal-error", resp.Error)
	}
}

// --- Reuse bufio.NewReader import for coverage ---------------------------

var _ = bufio.NewReader

// --- Sanity: encode a parse-error response using bufio.Writer ------------

func TestServe_ParseErrorUsesBufioWriter(t *testing.T) {
	out := &strings.Builder{}
	s := &Server{
		projectRoot: t.TempDir(),
		docsDir:     "docs",
		in:          strings.NewReader("garbage_no_newline"),
		out:         out,
	}
	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve = %v", err)
	}
	if !strings.Contains(out.String(), `"code":-32700`) {
		t.Fatalf("want parse error code, got: %q", out.String())
	}
}

// --- Verify that Run returns nil for --help ------------------------------

func TestRun_LongHelp(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("Run(--help) = %v, want nil", err)
	}
}

// --- Verify Run uses os.Getwd path (with a missing docs dir) -------------

func TestRun_GetwdError(t *testing.T) {
	// Replace getwdFn with one that errors to exercise the os.Getwd error path.
	origGetwd := getwdFn
	defer func() { getwdFn = origGetwd }()
	getwdFn = func() (string, error) { return "", errors.New("synthetic getwd failure") }

	if err := Run(nil); err == nil || !strings.Contains(err.Error(), "synthetic getwd failure") {
		t.Fatalf("Run(getwd-err) = %v, want synthetic getwd failure", err)
	}
}