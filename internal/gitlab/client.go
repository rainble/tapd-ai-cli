// Package gitlab 提供 GitLab REST API 的最小客户端。
package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client 是 GitLab REST API 客户端。
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Issue 表示 GitLab issue 创建接口返回的核心字段。
type Issue struct {
	ID        int    `json:"id"`
	IID       int    `json:"iid"`
	WebURL    string `json:"web_url"`
	ProjectID int    `json:"project_id"`
}

// Note 表示 GitLab issue note 创建接口返回的核心字段。
type Note struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// CreateIssueRequest 是创建 GitLab issue 的请求。
type CreateIssueRequest struct {
	Title        string
	Description  string
	Labels       []string
	AssigneeIDs  []int
	DueDate      string
	Confidential bool
	IssueType    string
}

// APIError 表示 GitLab API 返回的非 2xx 错误。
type APIError struct {
	StatusCode int
	Body       string
}

// Error 返回可读错误信息。
func (e *APIError) Error() string {
	return fmt.Sprintf("gitlab api error: status=%d body=%s", e.StatusCode, e.Body)
}

// AsAPIError 判断 err 是否为 APIError。
func AsAPIError(err error, target **APIError) bool {
	return errors.As(err, target)
}

// NewClient 创建 GitLab API 客户端。
func NewClient(baseURL, token string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	return &Client{
		baseURL:    baseURL,
		token:      strings.TrimSpace(token),
		httpClient: http.DefaultClient,
	}
}

// CreateIssue 创建 GitLab issue。
func (c *Client) CreateIssue(ctx context.Context, project string, req CreateIssueRequest) (*Issue, error) {
	values := url.Values{}
	values.Set("title", req.Title)
	if req.Description != "" {
		values.Set("description", req.Description)
	}
	if len(req.Labels) > 0 {
		values.Set("labels", strings.Join(req.Labels, ","))
	}
	if len(req.AssigneeIDs) > 0 {
		values.Set("assignee_ids", joinInts(req.AssigneeIDs))
	}
	if req.DueDate != "" {
		values.Set("due_date", req.DueDate)
	}
	if req.Confidential {
		values.Set("confidential", "true")
	}
	if req.IssueType != "" {
		values.Set("issue_type", req.IssueType)
	}

	var issue Issue
	err := c.postForm(ctx, "/api/v4/projects/"+escapeProject(project)+"/issues", values, &issue)
	return &issue, err
}

// CreateIssueNote 给 GitLab issue 追加 note。
func (c *Client) CreateIssueNote(ctx context.Context, project string, issueIID int, body string) (*Note, error) {
	values := url.Values{}
	values.Set("body", body)
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%d/notes", escapeProject(project), issueIID)
	var note Note
	err := c.postForm(ctx, path, values, &note)
	return &note, err
}

func (c *Client) postForm(ctx context.Context, path string, values url.Values, out interface{}) error {
	if c.token == "" {
		return fmt.Errorf("gitlab token is required")
	}
	target := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

func escapeProject(project string) string {
	return url.PathEscape(strings.TrimSpace(project))
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, ",")
}
