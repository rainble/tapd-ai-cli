// Package mcp 实现 Model Context Protocol (MCP) stdio server。
//
// 协议层：JSON-RPC 2.0 over newline-delimited JSON on stdio。
// 业务层：把 tapd-sdk-go 的方法以 MCP tool 形式暴露给 AI 客户端
// （Claude Code、Cursor、Codex 等）。
//
// 实现选择：不引入 MCP SDK，直接处理协议——好处是零额外依赖，坏处是
// 协议演进时需要手动跟进。当前覆盖：initialize / tools/list / tools/call /
// notifications/initialized。已足够 Claude Code 与 Cursor 接入。
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	tapd "github.com/studyzy/tapd-sdk-go"
)

// 协议常量。当前协议版本沿用 2024-11-05 spec，后续如需升级单点替换即可。
const (
	protocolVersion = "2024-11-05"
	serverName      = "tapd-ai-cli"
)

// rpcRequest 表示一条 JSON-RPC 入参。
// 注意 ID 用 json.RawMessage：JSON-RPC 允许 number / string / null 三种类型，
// 而通知（notification）不带 ID——保留原始字节最稳。
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse 表示一条 JSON-RPC 响应；result 与 error 互斥。
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError 是 JSON-RPC 标准错误对象。
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// 标准错误码沿用 JSON-RPC 2.0 规范定义的子集。
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// ToolHandler 实现一个 MCP 工具的执行逻辑，args 是 tools/call 透传过来的 arguments JSON。
// 返回的 interface{} 会被序列化成 MCP content 数组中的一条 text item。
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// ResourceHandler 实现一个 MCP resource 的读取逻辑，uri 是 resources/read 请求的资源标识。
// 返回的 interface{} 会被序列化成 MCP contents 数组。
type ResourceHandler func(ctx context.Context, uri string) (interface{}, error)

// Tool 描述一个 MCP 工具的元信息和执行体。
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Handler     ToolHandler     `json:"-"`
	AllowNoTAPD bool            `json:"-"`
}

// Resource 描述一个 MCP 资源的元信息和读取体。
type Resource struct {
	URI         string          `json:"uri"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	MimeType    string          `json:"mimeType,omitempty"`
	Handler     ResourceHandler `json:"-"`
}

// Server 持有 stdio 流、tapd 客户端和已注册工具表。
type Server struct {
	in        *bufio.Reader
	out       *json.Encoder
	logger    io.Writer // stderr，用于结构化日志，不能写到 stdout（会污染 JSON-RPC）
	client    *tapd.Client
	tools     map[string]*Tool
	resources map[string]*Resource

	mu     sync.Mutex // 保护 out 写入；handler 内可能并发触发响应
	closed bool
}

// NewServer 构造 server。stdin/stdout 是 MCP 协议必须的，stderr 用于诊断。
func NewServer(in io.Reader, out io.Writer, logger io.Writer, client *tapd.Client) *Server {
	return &Server{
		in:        bufio.NewReaderSize(in, 64*1024),
		out:       json.NewEncoder(out),
		logger:    logger,
		client:    client,
		tools:     make(map[string]*Tool),
		resources: make(map[string]*Resource),
	}
}

// Client 暴露给 tools 包构造工具时使用。
func (s *Server) Client() *tapd.Client { return s.client }

// Register 注册一个工具；重名直接覆盖（方便测试桩）。
func (s *Server) Register(t *Tool) {
	s.tools[t.Name] = t
}

// RegisterResource 注册一个 resource；重名直接覆盖。
func (s *Server) RegisterResource(r *Resource) {
	s.resources[r.URI] = r
}

// Run 主循环：读一行 → 解析 → 路由方法 → 写一行响应。
// 任何 read 错误（含 EOF）都会优雅退出，stdin 关闭即视为客户端断开。
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := s.in.ReadBytes('\n')
		if len(line) > 0 {
			s.dispatch(ctx, line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// dispatch 解析一条请求并路由到 handleXxx；handler 内部负责写响应。
// 解析失败也尽力回写错误，让客户端知道问题所在。
func (s *Server) dispatch(ctx context.Context, raw []byte) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		s.writeError(nil, errParseError, fmt.Sprintf("parse error: %v", err))
		return
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		s.writeError(req.ID, errInvalidRequest, "jsonrpc version must be 2.0")
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// 客户端确认握手完成，无需响应（这是个通知）
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourcesRead(ctx, req)
	case "ping":
		s.writeResult(req.ID, map[string]interface{}{})
	default:
		// 通知没 ID，按规范不应回响应
		if len(req.ID) == 0 {
			return
		}
		s.writeError(req.ID, errMethodNotFound, "method not found: "+req.Method)
	}
}

// handleInitialize 协议握手：返回服务端能力 + 自我介绍。
func (s *Server) handleInitialize(req rpcRequest) {
	s.writeResult(req.ID, map[string]interface{}{
		"protocolVersion": protocolVersion,
		"serverInfo": map[string]interface{}{
			"name":    serverName,
			"version": "0.1.0",
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
		},
	})
}

// handleToolsList 返回所有注册工具，名字按字典序输出，便于客户端展示稳定。
func (s *Server) handleToolsList(req rpcRequest) {
	list := make([]*Tool, 0, len(s.tools))
	for _, t := range s.tools {
		list = append(list, t)
	}
	// 简单排序，避免 map 顺序导致客户端 UI 闪动
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].Name > list[j].Name {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	s.writeResult(req.ID, map[string]interface{}{"tools": list})
}

// handleToolsCall 执行一个具体工具，把返回值包装成 MCP content。
//
// 当前所有工具都需要 TAPD client；client 缺失时直接以 isError content 回包，
// 不让请求穿透到 handler。这样客户端只把"凭据缺失"当成可恢复错误，不影响协议握手。
func (s *Server) handleToolsCall(ctx context.Context, req rpcRequest) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.writeError(req.ID, errInvalidParams, "invalid params: "+err.Error())
		return
	}
	tool, ok := s.tools[p.Name]
	if !ok {
		s.writeError(req.ID, errMethodNotFound, "tool not found: "+p.Name)
		return
	}
	if s.client == nil && !tool.AllowNoTAPD {
		s.writeIsErrorText(req.ID,
			"TAPD credentials not configured: please run 'tapd auth login --access-token <token>' "+
				"(or set TAPD_ACCESS_TOKEN), then restart the MCP server.")
		return
	}
	result, err := tool.Handler(ctx, p.Arguments)
	if err != nil {
		// 工具内部错误按 MCP 约定回包成 isError=true 的 content，
		// 这样模型能看到错误信息并自行重试或换工具调用，而不是 RPC 层错误中断。
		s.writeIsErrorText(req.ID, err.Error())
		return
	}
	text, err := stringifyResult(result)
	if err != nil {
		s.writeError(req.ID, errInternal, "marshal result: "+err.Error())
		return
	}
	s.writeResult(req.ID, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": text},
		},
	})
}

// writeIsErrorText 写一条 isError=true 的 MCP content 响应。
func (s *Server) writeIsErrorText(id json.RawMessage, text string) {
	s.writeResult(id, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": text},
		},
		"isError": true,
	})
}

// stringifyResult 把任意 result 转成给模型看的字符串：
// 字符串原样、其他类型走紧凑 JSON——节省 token，不浪费缩进。
func stringifyResult(v interface{}) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// writeResult 写一条成功响应。线程安全。
func (s *Server) writeResult(id json.RawMessage, result interface{}) {
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

// writeError 写一条错误响应；id 为空时用 null（兼容 parse error 场景）。
func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) write(resp rpcResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if err := s.out.Encode(resp); err != nil {
		fmt.Fprintf(s.logger, "mcp: write response err: %v\n", err)
	}
}

// handleResourcesList 返回所有注册的 resources。
func (s *Server) handleResourcesList(req rpcRequest) {
	list := make([]*Resource, 0, len(s.resources))
	for _, r := range s.resources {
		list = append(list, r)
	}
	// 按 URI 字典序排序,便于客户端展示稳定
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].URI > list[j].URI {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	s.writeResult(req.ID, map[string]interface{}{"resources": list})
}

// handleResourcesRead 读取一个具体 resource 的内容。
func (s *Server) handleResourcesRead(ctx context.Context, req rpcRequest) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.writeError(req.ID, errInvalidParams, "invalid params: "+err.Error())
		return
	}
	res, ok := s.resources[p.URI]
	if !ok {
		s.writeError(req.ID, errMethodNotFound, "resource not found: "+p.URI)
		return
	}
	content, err := res.Handler(ctx, p.URI)
	if err != nil {
		// resource 读取失败按 MCP 约定回 isError content,让模型能看到错误
		s.writeIsErrorText(req.ID, err.Error())
		return
	}
	text, err := stringifyResult(content)
	if err != nil {
		s.writeError(req.ID, errInternal, "marshal resource content: "+err.Error())
		return
	}
	s.writeResult(req.ID, map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"uri":      p.URI,
				"mimeType": res.MimeType,
				"text":     text,
			},
		},
	})
}
