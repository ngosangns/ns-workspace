// Package kbmcp implements a local-only Model Context Protocol (MCP) server
// that exposes a project's docs knowledge base over JSON-RPC 2.0 via stdin/
// stdout. It never binds a network port: the server is meant to be spawned by
// an MCP-capable agent as a stdio subprocess, so its blast radius is limited to
// the project's docs root.
//
// This file provides the transport scaffolding: flag parsing (Run), the Server
// type (NewServer/Serve), the JSON-RPC read/dispatch/write loop, panic recovery
// into JSON-RPC error responses, and graceful handling of unknown methods and
// tools (return an error, never crash the loop). The concrete tool handlers
// (handleListDocs / handleLookupDoc / handleSearchDocs / handleModifyDoc) live
// in tools.go and are filled in by later tasks; here they are stubs that return
// a "not implemented" JSON-RPC error.
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
)

// JSON-RPC 2.0 standard error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

const jsonRPCVersion = "2.0"

// rpcRequest is an incoming JSON-RPC 2.0 request (or notification when ID is
// absent). Params is kept raw so each method can decode its own shape.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is an outgoing JSON-RPC 2.0 response. Exactly one of Result or
// Error is populated.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// toolCallParams is the params shape for the MCP "tools/call" method.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Server holds the project root and docs directory so every tool operates on
// the same knowledge base, plus the stdio streams used for JSON-RPC framing.
type Server struct {
	projectRoot string
	docsDir     string
	in          io.Reader
	out         io.Writer
}

// NewServer builds a Server bound to the given project root and docs directory.
// It reads JSON-RPC requests from os.Stdin and writes responses to os.Stdout;
// tests may override in/out via the unexported fields.
func NewServer(projectRoot, docsDir string) *Server {
	return &Server{
		projectRoot: projectRoot,
		docsDir:     docsDir,
		in:          os.Stdin,
		out:         os.Stdout,
	}
}

// getwdFn is a test seam for os.Getwd.
var getwdFn = os.Getwd

// routeFn is a test seam for the route function. It allows tests to inject
// panicking handlers to exercise the panic recovery branch in dispatch.
var routeFn = func(s *Server, ctx context.Context, req rpcRequest) rpcResponse {
	return s.route(ctx, req)
}

// Run parses flags and starts the MCP stdio server. It supports --project and
// --docs (aliased as --docs-dir to match the other preview/search/graph
// commands). The server is local-only and never binds a network port.
func Run(args []string) error {
	cwd, err := getwdFn()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"

	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root to inspect")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&docsDir, "docs", docsDir, "alias for --docs-dir")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	server := NewServer(projectRoot, docsDir)
	return server.Serve(context.Background())
}

// Serve runs the JSON-RPC read→dispatch→write loop over the configured stdio
// streams until EOF or context cancellation. Each request is dispatched in
// isolation; a panic or unknown method/tool produces a JSON-RPC error response
// and the loop keeps serving.
func (s *Server) Serve(ctx context.Context) error {
	reader := bufio.NewReader(s.in)
	writer := bufio.NewWriter(s.out)
	dec := json.NewDecoder(reader)
	enc := json.NewEncoder(writer)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// The stream is malformed and json.Decoder cannot resync, so emit a
			// final parse-error response and stop rather than spinning forever.
			_ = enc.Encode(rpcResponse{
				JSONRPC: jsonRPCVersion,
				ID:      jsonNull(),
				Error:   &rpcError{Code: codeParseError, Message: err.Error()},
			})
			_ = writer.Flush()
			return nil
		}

		resp, send := s.dispatch(ctx, req)
		if !send {
			// Notifications (no id) get no response.
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
}

// dispatch routes a single request and reports whether a response should be
// written. Notifications (requests without an id) are handled silently. Any
// panic inside a handler is recovered and converted into a JSON-RPC error.
func (s *Server) dispatch(ctx context.Context, req rpcRequest) (resp rpcResponse, send bool) {
	if isNotification(req) {
		return rpcResponse{}, false
	}

	defer func() {
		if r := recover(); r != nil {
			resp = s.errorResponse(req.ID, codeInternalError, fmt.Sprintf("handler panic: %v", r))
			send = true
		}
	}()

	return routeFn(s, ctx, req), true
}

// route maps a JSON-RPC method to its handler. Unknown methods return a
// method-not-found error instead of crashing.
func (s *Server) route(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return s.okResponse(req.ID, s.initializeResult())
	case "tools/list":
		return s.okResponse(req.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return s.errorResponse(req.ID, codeMethodNotFound, fmt.Sprintf("unsupported method: %q", req.Method))
	}
}

// handleToolCall decodes the tools/call params and dispatches to the named tool
// handler. Invalid params or an unknown tool name return a JSON-RPC error; the
// server keeps serving.
func (s *Server) handleToolCall(ctx context.Context, req rpcRequest) rpcResponse {
	var params toolCallParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, codeInvalidParams, fmt.Sprintf("invalid tools/call params: %v", err))
		}
	}

	var (
		result any
		err    error
	)
	switch params.Name {
	case "list_docs":
		result, err = s.handleListDocs(params.Arguments)
	case "lookup_doc":
		result, err = s.handleLookupDoc(params.Arguments)
	case "search_docs":
		result, err = s.handleSearchDocs(ctx, params.Arguments)
	case "modify_doc":
		result, err = s.handleModifyDoc(params.Arguments)
	default:
		return s.errorResponse(req.ID, codeInvalidParams, fmt.Sprintf("unknown tool: %q", params.Name))
	}
	if err != nil {
		return s.errorResponse(req.ID, codeInternalError, err.Error())
	}
	return s.okResponse(req.ID, result)
}

// initializeResult returns the minimal MCP initialize result advertising the
// tools capability.
func (s *Server) initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "ns-workspace-kbmcp",
			"version": "0.1.0",
		},
	}
}

// okResponse builds a success response carrying result.
func (s *Server) okResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: normalizeID(id), Result: result}
}

// errorResponse builds an error response with the given code and message.
func (s *Server) errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      normalizeID(id),
		Error:   &rpcError{Code: code, Message: message},
	}
}

// isNotification reports whether a request is a JSON-RPC notification (no id).
func isNotification(req rpcRequest) bool {
	if len(req.ID) == 0 {
		return true
	}
	return string(req.ID) == "null"
}

// normalizeID returns the request id, defaulting to JSON null when absent so the
// response always carries an explicit id field per JSON-RPC 2.0.
func normalizeID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return jsonNull()
	}
	return id
}

func jsonNull() json.RawMessage {
	return json.RawMessage("null")
}
