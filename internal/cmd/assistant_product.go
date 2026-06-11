package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-ai-cli/internal/tapdurl"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagAssistantProductInput   string
	flagAssistantProductFile    string
	flagAssistantProductComment bool
)

var assistantCmd = &cobra.Command{
	Use:   "assistant",
	Short: "角色化 TAPD AI 工作助手",
}

var assistantProductCmd = &cobra.Command{
	Use:   "product",
	Short: "产品提需助手",
}

var assistantProductDraftStoryCmd = &cobra.Command{
	Use:   "draft-story",
	Short: "从粗略想法生成结构化需求草稿",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProductDraftStory(cmdContext(cmd)); err != nil {
			output.PrintError(os.Stderr, "assistant_product_error", err.Error(), "")
			os.Exit(output.ExitParamError)
		}
		return nil
	},
}

var assistantProductCheckStoryCmd = &cobra.Command{
	Use:   "check-story <story_id_or_url>",
	Short: "检查 TAPD 需求材料是否完整",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProductCheckStory(cmdContext(cmd), args, false); err != nil {
			output.PrintError(os.Stderr, "assistant_product_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

var assistantProductReviewReadyCmd = &cobra.Command{
	Use:   "review-ready <story_id_or_url>",
	Short: "判断 TAPD 需求是否达到评审条件",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProductCheckStory(cmdContext(cmd), args, true); err != nil {
			output.PrintError(os.Stderr, "assistant_product_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

func init() {
	assistantProductDraftStoryCmd.Flags().StringVar(&flagAssistantProductInput, "input", "", "粗略需求想法")
	assistantProductDraftStoryCmd.Flags().StringVar(&flagAssistantProductFile, "file", "", "从本地文件读取粗略需求想法")
	assistantProductDraftStoryCmd.Flags().BoolVar(&flagJSON, "json", false, "输出 JSON")

	for _, c := range []*cobra.Command{assistantProductCheckStoryCmd, assistantProductReviewReadyCmd} {
		c.Flags().BoolVar(&flagAssistantProductComment, "comment", false, "将检查结果写入 TAPD 评论")
		c.Flags().BoolVar(&flagJSON, "json", false, "输出 JSON")
	}

	assistantProductCmd.AddCommand(assistantProductDraftStoryCmd, assistantProductCheckStoryCmd, assistantProductReviewReadyCmd)
	assistantCmd.AddCommand(assistantProductCmd)
	rootCmd.AddCommand(assistantCmd)
}

type productRequirementContext struct {
	Story    productStorySnapshot     `json:"story"`
	Comments []productCommentSnapshot `json:"comments,omitempty"`
}

type productStorySnapshot struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner,omitempty"`
	Developer   string `json:"developer,omitempty"`
	Priority    string `json:"priority,omitempty"`
	IterationID string `json:"iteration_id,omitempty"`
	CategoryID  string `json:"category_id,omitempty"`
	Module      string `json:"module,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

type productCommentSnapshot struct {
	Author      string `json:"author,omitempty"`
	Created     string `json:"created,omitempty"`
	Description string `json:"description,omitempty"`
}

type productRequirementReport struct {
	StoryID        string                    `json:"story_id,omitempty"`
	StoryName      string                    `json:"story_name,omitempty"`
	Ready          bool                      `json:"ready"`
	Score          int                       `json:"score"`
	BlockingIssues []productRequirementIssue `json:"blocking_issues"`
	Warnings       []productRequirementIssue `json:"warnings"`
	Suggestions    []string                  `json:"suggestions"`
	Summary        string                    `json:"summary"`
}

type productRequirementIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func runAssistantProductDraftStory(ctx context.Context) error {
	input, err := readAssistantProductDraftInput()
	if err != nil {
		return err
	}
	draft := buildProductRequirementDraft(input)
	if flagJSON {
		return output.PrintJSON(os.Stdout, map[string]string{"draft": draft}, !flagPretty)
	}
	_, err = fmt.Fprintln(os.Stdout, draft)
	return err
}

func readAssistantProductDraftInput() (string, error) {
	hasInput := strings.TrimSpace(flagAssistantProductInput) != ""
	hasFile := strings.TrimSpace(flagAssistantProductFile) != ""
	if hasInput == hasFile {
		return "", fmt.Errorf("provide exactly one of --input or --file")
	}
	if hasInput {
		return strings.TrimSpace(flagAssistantProductInput), nil
	}
	data, err := os.ReadFile(flagAssistantProductFile)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("--file content is empty")
	}
	return strings.TrimSpace(string(data)), nil
}

func runAssistantProductCheckStory(ctx context.Context, args []string, reviewOnly bool) error {
	workspaceID, storyID, err := resolveProductStoryRef(args[0])
	if err != nil {
		return err
	}
	ctxData, err := loadProductRequirementContext(ctx, workspaceID, storyID)
	if err != nil {
		return err
	}
	report := checkProductRequirement(ctxData)
	if flagJSON {
		if err := output.PrintJSON(os.Stdout, report, !flagPretty); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(os.Stdout, renderProductRequirementReportMarkdown(report, reviewOnly)); err != nil {
			return err
		}
	}
	if flagAssistantProductComment {
		return addProductRequirementComment(ctx, workspaceID, storyID, renderProductRequirementReportMarkdown(report, reviewOnly))
	}
	return nil
}

func resolveProductStoryRef(raw string) (workspaceID, storyID string, err error) {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := tapdurl.Parse(raw)
		if err != nil {
			return "", "", err
		}
		if parsed.EntityType != "story" {
			return "", "", fmt.Errorf("expected story URL, got %s", parsed.EntityType)
		}
		return parsed.WorkspaceID, parsed.EntityID, nil
	}
	return flagWorkspaceID, expandShortID(raw, flagWorkspaceID), nil
}

func loadProductRequirementContext(ctx context.Context, workspaceID, storyID string) (productRequirementContext, error) {
	story, err := apiClient.GetStory(ctx, workspaceID, storyID)
	if err != nil {
		return productRequirementContext{}, err
	}
	comments, commentErr := apiClient.ListComments(ctx, &model.ListCommentsRequest{
		WorkspaceID: workspaceID,
		EntryType:   "stories",
		EntryID:     storyID,
		Limit:       50,
		Order:       "created desc",
	})
	out := productRequirementContext{
		Story: productStorySnapshot{
			WorkspaceID: workspaceID,
			ID:          story.ID,
			Name:        story.Name,
			Description: htmlToMarkdown(firstNonEmpty(story.MarkdownDescription, story.Description)),
			Owner:       story.Owner,
			Developer:   story.Developer,
			Priority:    story.PriorityLabel,
			IterationID: story.IterationID,
			CategoryID:  story.CategoryID,
			Module:      story.Module,
			ParentID:    story.ParentID,
		},
	}
	if commentErr == nil {
		for _, c := range comments {
			out.Comments = append(out.Comments, productCommentSnapshot{
				Author:      c.Author,
				Created:     c.Created,
				Description: htmlToMarkdown(c.Description),
			})
		}
	}
	return out, nil
}

func addProductRequirementComment(ctx context.Context, workspaceID, storyID, description string) error {
	_, err := apiClient.AddComment(ctx, &model.AddCommentRequest{
		WorkspaceID: workspaceID,
		EntryType:   "stories",
		EntryID:     storyID,
		Description: description,
		Author:      ensureNick(),
	})
	return err
}

func buildProductRequirementDraft(input string) string {
	input = strings.TrimSpace(input)
	return fmt.Sprintf(`# Product Requirement Draft

## Background
%s

## Goal
- Clarify the measurable outcome this requirement should achieve.

## User Value
- Describe who benefits and what problem is solved.

## Scope
- Include the product behaviors, pages, data, or workflows covered by this requirement.

## Non-Scope
- List what is intentionally excluded from this iteration.

## Functional Requirements
- Requirement 1:
- Requirement 2:

## Acceptance Criteria
- Given ..., when ..., then ...

## Data Requirements
- Metrics:
- Data sources:
- Reporting or dashboard needs:

## Dependencies
- Upstream/downstream systems:
- Reviewers or partner teams:

## Risks
- Product risk:
- Technical risk:
- Launch risk:

## Rollout Plan
- Release strategy:
- Monitoring:
- Fallback:

## Open Questions
- What details are still missing before review?
`, input)
}

func checkProductRequirement(ctx productRequirementContext) productRequirementReport {
	report := productRequirementReport{
		StoryID:   ctx.Story.ID,
		StoryName: ctx.Story.Name,
		Ready:     true,
		Score:     100,
	}
	addBlocker := func(code, message string) {
		report.BlockingIssues = append(report.BlockingIssues, productRequirementIssue{Code: code, Message: message})
		report.Ready = false
		report.Score -= 18
	}
	addWarning := func(code, message string) {
		report.Warnings = append(report.Warnings, productRequirementIssue{Code: code, Message: message})
		report.Score -= 6
	}

	description := ctx.Story.Description
	if strings.TrimSpace(ctx.Story.Name) == "" {
		addBlocker("missing_name", "Missing story name.")
	}
	if strings.TrimSpace(description) == "" {
		addBlocker("missing_description", "Missing requirement description.")
	}
	if strings.TrimSpace(ctx.Story.Owner) == "" {
		addBlocker("missing_owner", "Missing requirement owner.")
	}
	if !containsAny(description, "background", "背景") {
		addBlocker("missing_background", "Missing background.")
	}
	if !containsAny(description, "goal", "目标") {
		addBlocker("missing_goal", "Missing goal.")
	}
	if !containsAny(description, "scope", "范围") {
		addBlocker("missing_scope", "Missing scope.")
	}
	if !containsAny(description, "acceptance criteria", "验收", "验收标准") {
		addBlocker("missing_acceptance_criteria", "Missing acceptance criteria.")
	}
	if !containsAny(description, "non-scope", "不包含", "非范围") {
		addWarning("missing_non_scope", "Non-scope is not described.")
	}
	if !containsAny(description, "risk", "风险") {
		addWarning("missing_risks", "Risks are not described.")
	}
	if !containsAny(description, "dependency", "dependencies", "依赖") {
		addWarning("missing_dependencies", "Dependencies are not described.")
	}
	if report.Score < 0 {
		report.Score = 0
	}
	report.Suggestions = buildProductRequirementSuggestions(report)
	report.Summary = buildProductRequirementSummary(report)
	return report
}

func containsAny(text string, needles ...string) bool {
	text = strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func buildProductRequirementSuggestions(report productRequirementReport) []string {
	var suggestions []string
	for _, issue := range report.BlockingIssues {
		switch issue.Code {
		case "missing_acceptance_criteria":
			suggestions = append(suggestions, "Add concrete acceptance criteria so reviewers, developers, and testers can verify completion.")
		case "missing_background":
			suggestions = append(suggestions, "Add background explaining why this requirement exists now.")
		case "missing_goal":
			suggestions = append(suggestions, "Add a measurable goal for the requirement.")
		case "missing_scope":
			suggestions = append(suggestions, "Clarify what is included in this iteration.")
		case "missing_owner":
			suggestions = append(suggestions, "Set the TAPD story owner before review.")
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Requirement material is ready for review. Keep comments and changes synchronized in TAPD.")
	}
	return suggestions
}

func buildProductRequirementSummary(report productRequirementReport) string {
	if report.Ready {
		return "Requirement appears ready for review."
	}
	return fmt.Sprintf("Requirement is not ready for review. %d blocking issue(s) need attention.", len(report.BlockingIssues))
}

func renderProductRequirementReportMarkdown(report productRequirementReport, reviewOnly bool) string {
	title := "Requirement Readiness Report"
	if reviewOnly {
		title = "Requirement Review Readiness Decision"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	if report.StoryID != "" || report.StoryName != "" {
		fmt.Fprintf(&b, "Story: %s %s\n", report.StoryID, report.StoryName)
	}
	fmt.Fprintf(&b, "Ready: %s\nScore: %d\n\n", yesNo(report.Ready), report.Score)
	writeIssuesMarkdown(&b, "Blocking Issues", report.BlockingIssues)
	writeIssuesMarkdown(&b, "Warnings", report.Warnings)
	if len(report.Suggestions) > 0 {
		b.WriteString("## Suggestions\n")
		for _, s := range report.Suggestions {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Summary\n%s\n", report.Summary)
	return b.String()
}

func writeIssuesMarkdown(w io.Writer, title string, issues []productRequirementIssue) {
	fmt.Fprintf(w, "## %s\n", title)
	if len(issues) == 0 {
		fmt.Fprintln(w, "- None")
	} else {
		for _, issue := range issues {
			fmt.Fprintf(w, "- %s\n", issue.Message)
		}
	}
	fmt.Fprintln(w)
}

func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func marshalProductReport(report productRequirementReport) string {
	data, _ := json.Marshal(report)
	return string(data)
}
