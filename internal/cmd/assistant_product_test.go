package cmd

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestProductRequirementDraftIncludesStandardSections(t *testing.T) {
	draft := buildProductRequirementDraft("希望支持主播付费视频自动续费签约数据看板")
	for _, want := range []string{
		"# Product Requirement Draft",
		"主播付费视频自动续费签约数据看板",
		"## Background",
		"## Goal",
		"## Acceptance Criteria",
		"## Open Questions",
	} {
		if !strings.Contains(draft, want) {
			t.Fatalf("draft missing %q:\n%s", want, draft)
		}
	}
}

func TestProductRequirementCheckBlocksMissingAcceptanceCriteria(t *testing.T) {
	report := checkProductRequirement(productRequirementContext{
		Story: productStorySnapshot{
			ID:          "10001",
			Name:        "自动续费签约数据看板",
			Description: "## Background\n需要看签约数据。\n\n## Goal\n提升运营效率。\n\n## Scope\n新增数据看板。",
			Owner:       "alice",
		},
	})
	if report.Ready {
		t.Fatalf("report should not be ready: %+v", report)
	}
	if report.Score <= 0 || report.Score >= 100 {
		t.Fatalf("score=%d, want partial score", report.Score)
	}
	if !productIssueCodes(report.BlockingIssues).Has("missing_acceptance_criteria") {
		t.Fatalf("blocking issues=%+v", report.BlockingIssues)
	}
	md := renderProductRequirementReportMarkdown(report, false)
	if !strings.Contains(md, "Requirement Readiness Report") || !strings.Contains(md, "Missing acceptance criteria") {
		t.Fatalf("markdown report unexpected:\n%s", md)
	}
}

func TestRunAssistantProductCheckStoryWritesCommentWhenRequested(t *testing.T) {
	resetFlags()
	t.Cleanup(resetFlags)

	var postForms []url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Method == http.MethodPost {
			cp := url.Values{}
			for k, v := range r.PostForm {
				cp[k] = append([]string(nil), v...)
			}
			postForms = append(postForms, cp)
		}
		switch {
		case strings.Contains(r.URL.Path, "/comments") && r.Method == http.MethodPost:
			w.Write([]byte(`{"status":1,"data":{"Comment":{"id":"c2","author":"agent","description":"ok"}}}`))
		case strings.Contains(r.URL.Path, "/comments"):
			w.Write([]byte(`{"status":1,"data":[{"Comment":{"id":"c1","author":"bob","description":"<p>补充一下数据口径</p>"}}]}`))
		case strings.Contains(r.URL.Path, "/stories"):
			w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"456","name":"数据看板","description":"<h2>Background</h2><p>需要数据看板</p><h2>Goal</h2><p>提升运营效率</p><h2>Scope</h2><p>新增看板</p>","owner":"alice"}}]}`))
		default:
			w.Write([]byte(`{"status":1,"data":[]}`))
		}
	}
	_, cleanup := setupMockServer(t, handler)
	defer cleanup()
	apiClient.SetNick("agent")
	flagAssistantProductComment = true

	restore, reader := captureStdout(t)
	err := runAssistantProductCheckStory(context.Background(), []string{"456"}, false)
	restore()
	out, _ := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Requirement Readiness Report") {
		t.Fatalf("stdout=%s", out)
	}
	if len(postForms) != 1 {
		t.Fatalf("postForms=%v", postForms)
	}
	if postForms[0].Get("entry_type") != "stories" || postForms[0].Get("entry_id") != "1112345000000456" {
		t.Fatalf("comment form=%v", postForms[0])
	}
	if !strings.Contains(postForms[0].Get("description"), "Requirement Readiness Report") {
		t.Fatalf("comment form=%v", postForms[0])
	}
}

func TestRunAssistantProductDraftStoryRequiresOneInput(t *testing.T) {
	resetFlags()
	t.Cleanup(resetFlags)
	if err := runAssistantProductDraftStory(context.Background()); err == nil {
		t.Fatal("expected missing input error")
	}
	flagAssistantProductInput = "idea"
	flagAssistantProductFile = "file.md"
	if err := runAssistantProductDraftStory(context.Background()); err == nil {
		t.Fatal("expected input conflict error")
	}
}

type productIssueCodeSet []productRequirementIssue

func productIssueCodes(issues []productRequirementIssue) productIssueCodeSet {
	return productIssueCodeSet(issues)
}

func (s productIssueCodeSet) Has(code string) bool {
	for _, issue := range s {
		if issue.Code == code {
			return true
		}
	}
	return false
}

var _ = os.Stdout
