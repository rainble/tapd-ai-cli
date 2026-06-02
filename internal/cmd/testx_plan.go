// Package cmd 中的 testx_plan.go 实现了 testx plan 子模块（21 API）
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// plan 子命令独有的 flags
var (
	flagTxPlanWithStatistic bool
	flagTxPlanWithDetail    string
	flagTxPlanWithDescend   bool
	flagTxPlanWithAncestor  bool
	flagTxPlanName          string
	flagTxPlanArchive       string
	flagTxPlanStates        []string
	flagTxPlanItemType      string
	flagTxPlanIssueType     string
	flagTxPlanRelatedTypes  []string
	flagTxPlanStatus        string
	flagTxPlanSummary       string
	flagTxPlanBugId         string
)

// testxPlanCmd 是 testx plan 父命令
var testxPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "testx 测试计划模块（21 API）",
}

// ---- 子命令定义 ----

var testxPlanFolderCreateCmd = &cobra.Command{Use: "folder-create", Short: "创建计划目录", RunE: runTestxPlanFolderCreate}
var testxPlanFolderUpdateCmd = &cobra.Command{Use: "folder-update", Short: "更新计划目录", RunE: runTestxPlanFolderUpdate}
var testxPlanFolderChildrenCmd = &cobra.Command{Use: "folder-children", Short: "获取目录子信息", RunE: runTestxPlanFolderChildren}

var testxPlanGetCmd = &cobra.Command{Use: "get", Short: "获取计划详情", RunE: runTestxPlanGet}
var testxPlanCreateCmd = &cobra.Command{Use: "create", Short: "创建计划", RunE: runTestxPlanCreate}
var testxPlanUpdateCmd = &cobra.Command{Use: "update", Short: "更新计划", RunE: runTestxPlanUpdate}
var testxPlanUpdateScopeCmd = &cobra.Command{Use: "update-target-scope", Short: "更新计划范围目标", RunE: runTestxPlanUpdateScope}
var testxPlanBatchUpdateCaseCmd = &cobra.Command{Use: "batch-update-case", Short: "批量更新计划用例", RunE: runTestxPlanBatchUpdateCase}
var testxPlanBatchArchiveCmd = &cobra.Command{Use: "batch-archive", Short: "批量归档计划", RunE: runTestxPlanBatchArchive}
var testxPlanListCmd = &cobra.Command{Use: "list", Short: "目录下计划列表", RunE: runTestxPlanList}
var testxPlanCasesCmd = &cobra.Command{Use: "list-cases", Short: "计划下用例列表", RunE: runTestxPlanCases}
var testxPlanHistoryCmd = &cobra.Command{Use: "list-history", Short: "计划变更历史", RunE: runTestxPlanHistory}
var testxPlanStatsCmd = &cobra.Command{Use: "statistics", Short: "计划统计信息", RunE: runTestxPlanStats}

var testxPlanBugBindCmd = &cobra.Command{Use: "bind-bug", Short: "计划用例批量关联缺陷", RunE: runTestxPlanBugBind}
var testxPlanBugUnbindCmd = &cobra.Command{Use: "unbind-bug", Short: "移除计划用例关联缺陷", RunE: runTestxPlanBugUnbind}
var testxPlanCaseIssuesCmd = &cobra.Command{Use: "list-case-issues", Short: "计划下用例关联缺陷", RunE: runTestxPlanCaseIssues}
var testxPlanCaseEventsCmd = &cobra.Command{Use: "list-case-events", Short: "计划下用例事件", RunE: runTestxPlanCaseEvents}

var testxPlanBugsCmd = &cobra.Command{Use: "list-bugs", Short: "计划关联缺陷列表", RunE: runTestxPlanBugs}
var testxPlanBugStatsCmd = &cobra.Command{Use: "bug-statistics", Short: "计划关联缺陷统计", RunE: runTestxPlanBugStats}
var testxPlanStoriesCmd = &cobra.Command{Use: "list-stories", Short: "计划关联需求列表", RunE: runTestxPlanStories}
var testxPlanTemplatesCmd = &cobra.Command{Use: "list-templates", Short: "计划模板列表", RunE: runTestxPlanTemplates}

func init() {
	all := []*cobra.Command{
		testxPlanFolderCreateCmd, testxPlanFolderUpdateCmd, testxPlanFolderChildrenCmd,
		testxPlanGetCmd, testxPlanCreateCmd, testxPlanUpdateCmd, testxPlanUpdateScopeCmd,
		testxPlanBatchUpdateCaseCmd, testxPlanBatchArchiveCmd, testxPlanListCmd, testxPlanCasesCmd,
		testxPlanHistoryCmd, testxPlanStatsCmd,
		testxPlanBugBindCmd, testxPlanBugUnbindCmd, testxPlanCaseIssuesCmd, testxPlanCaseEventsCmd,
		testxPlanBugsCmd, testxPlanBugStatsCmd, testxPlanStoriesCmd, testxPlanTemplatesCmd,
	}
	for _, c := range all {
		testxAddDataFlag(c)
	}

	// folder-uid
	testxPlanFolderUpdateCmd.Flags().StringVar(&flagTxFolderUid, "folder-uid", "", "目录 UID（必填）")
	testxPlanListCmd.Flags().StringVar(&flagTxFolderUid, "folder-uid", "", "目录 UID（必填）")

	// folder-children: 父目录 uid 用 --uid
	testxPlanFolderChildrenCmd.Flags().StringVar(&flagTxUid, "uid", "", "父目录 UID（必填）")
	testxPlanFolderChildrenCmd.Flags().BoolVar(&flagTxPlanWithDescend, "with-descendant", false, "包含子节点")
	testxPlanFolderChildrenCmd.Flags().BoolVar(&flagTxPlanWithAncestor, "with-ancestor", false, "包含父节点")
	testxPlanFolderChildrenCmd.Flags().StringVar(&flagTxPlanName, "name", "", "按名称搜索")
	testxPlanFolderChildrenCmd.Flags().StringVar(&flagTxPlanArchive, "plan-archive", "", "归档状态")
	testxPlanFolderChildrenCmd.Flags().StringSliceVar(&flagTxPlanStates, "plan-states", nil, "计划状态（可重复）")
	testxPlanFolderChildrenCmd.Flags().StringVar(&flagTxPlanItemType, "item-type", "", "项类型")

	// uid（计划 uid）
	for _, c := range []*cobra.Command{testxPlanGetCmd, testxPlanUpdateCmd, testxPlanUpdateScopeCmd, testxPlanCasesCmd} {
		c.Flags().StringVar(&flagTxUid, "uid", "", "计划 UID（必填）")
	}
	testxPlanGetCmd.Flags().BoolVar(&flagTxPlanWithStatistic, "with-statistic", false, "返回统计信息")
	testxPlanGetCmd.Flags().StringVar(&flagTxPlanWithDetail, "with-detail", "", "返回详情字段")

	// plan-uid
	for _, c := range []*cobra.Command{
		testxPlanBatchUpdateCaseCmd, testxPlanHistoryCmd,
		testxPlanBugBindCmd, testxPlanBugUnbindCmd, testxPlanCaseIssuesCmd, testxPlanCaseEventsCmd,
		testxPlanBugsCmd, testxPlanStoriesCmd,
	} {
		c.Flags().StringVar(&flagTxPlanUid, "plan-uid", "", "计划 UID（必填）")
	}

	// case-uid
	for _, c := range []*cobra.Command{
		testxPlanBugUnbindCmd, testxPlanCaseIssuesCmd, testxPlanCaseEventsCmd,
	} {
		c.Flags().StringVar(&flagTxCaseUid, "case-uid", "", "用例 UID（必填）")
	}

	// issue-uid（仅 unbind）
	testxPlanBugUnbindCmd.Flags().StringVar(&flagTxIssueUid, "issue-uid", "", "缺陷 UID（必填）")

	// 分页
	for _, c := range []*cobra.Command{
		testxPlanHistoryCmd, testxPlanCaseIssuesCmd, testxPlanCaseEventsCmd,
		testxPlanBugsCmd, testxPlanStoriesCmd, testxPlanTemplatesCmd,
	} {
		testxAddPaginationFlags(c)
	}

	// list-case-issues 必填 type
	testxPlanCaseIssuesCmd.Flags().StringVar(&flagTxPlanIssueType, "issue-type", "", "issue 类型（必填）")

	// list-bugs 过滤
	testxPlanBugsCmd.Flags().StringSliceVar(&flagTxPlanRelatedTypes, "related-types", nil, "关联类型（多个用逗号分隔）")
	testxPlanBugsCmd.Flags().StringVar(&flagTxPlanStatus, "status", "", "状态")
	testxPlanBugsCmd.Flags().StringVar(&flagTxPlanSummary, "summary", "", "标题模糊查询")
	testxPlanBugsCmd.Flags().StringVar(&flagTxPlanBugId, "bug-id", "", "缺陷 ID")

	testxPlanCmd.AddCommand(all...)
}

// ---- 运行函数 ----

func runTestxPlanFolderCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxCreatePlanFolderRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, err := apiClient.TestxCreatePlanFolder(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanFolderUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("folder-uid", flagTxFolderUid) {
		return nil
	}
	req := &model.TestxUpdatePlanFolderRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.FolderUid = flagTxFolderUid
	r, err := apiClient.TestxUpdatePlanFolder(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanFolderChildren(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxListFolderChildrenRequest{
		Namespace:      flagTxNamespace,
		Uid:            flagTxUid,
		WithDescendant: flagTxPlanWithDescend,
		WithAncestor:   flagTxPlanWithAncestor,
		Name:           flagTxPlanName,
		PlanArchive:    flagTxPlanArchive,
		PlanStates:     flagTxPlanStates,
		ItemType:       flagTxPlanItemType,
	}
	r, err := apiClient.TestxListFolderChildren(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanGet(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxGetPlanRequest{
		Namespace:     flagTxNamespace,
		Uid:           flagTxUid,
		WithStatistic: flagTxPlanWithStatistic,
		WithDetail:    flagTxPlanWithDetail,
	}
	r, err := apiClient.TestxGetPlan(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxCreatePlanRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, err := apiClient.TestxCreatePlan(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxUpdatePlanRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.Uid = flagTxUid
	r, err := apiClient.TestxUpdatePlan(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanUpdateScope(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxUpdatePlanTargetScopeRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.Uid = flagTxUid
	r, err := apiClient.TestxUpdatePlanTargetScope(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanBatchUpdateCase(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) {
		return nil
	}
	req := &model.TestxBatchUpdatePlanCaseRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.PlanUid = flagTxPlanUid
	if err := apiClient.TestxBatchUpdatePlanCase(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxPlanBatchArchive(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxBatchArchivePlanRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	if err := apiClient.TestxBatchArchivePlan(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxPlanList(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("folder-uid", flagTxFolderUid) {
		return nil
	}
	req := &model.TestxListPlansRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.FolderUid = flagTxFolderUid
	r, total, err := apiClient.TestxListPlans(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanCases(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxListPlanCasesRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.Uid = flagTxUid
	r, total, err := apiClient.TestxListPlanCases(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanHistory(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) {
		return nil
	}
	req := &model.TestxListPlanHistoriesRequest{
		Namespace: flagTxNamespace,
		PlanUid:   flagTxPlanUid,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListPlanHistories(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanStats(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxPlanStatisticsRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, total, err := apiClient.TestxPlanStatistics(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanBugBind(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) {
		return nil
	}
	req := &model.TestxBatchBindPlanBugRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.PlanUid = flagTxPlanUid
	if err := apiClient.TestxBatchBindPlanBug(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxPlanBugUnbind(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) || !testxRequire("case-uid", flagTxCaseUid) || !testxRequire("issue-uid", flagTxIssueUid) {
		return nil
	}
	req := &model.TestxUnbindPlanBugRequest{
		Namespace: flagTxNamespace,
		PlanUid:   flagTxPlanUid,
		CaseUid:   flagTxCaseUid,
		IssueUid:  flagTxIssueUid,
	}
	if err := apiClient.TestxUnbindPlanBug(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxPlanCaseIssues(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) || !testxRequire("case-uid", flagTxCaseUid) || !testxRequire("issue-type", flagTxPlanIssueType) {
		return nil
	}
	req := &model.TestxListPlanCaseIssuesRequest{
		Namespace: flagTxNamespace,
		PlanUid:   flagTxPlanUid,
		CaseUid:   flagTxCaseUid,
		Type:      flagTxPlanIssueType,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListPlanCaseIssues(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanCaseEvents(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxListPlanCaseEventsRequest{
		Namespace: flagTxNamespace,
		PlanUid:   flagTxPlanUid,
		CaseUid:   flagTxCaseUid,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListPlanCaseEvents(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanBugs(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) {
		return nil
	}
	req := &model.TestxListPlanBugsRequest{
		Namespace:    flagTxNamespace,
		PlanUid:      flagTxPlanUid,
		PageInfo:     testxPageInfo(),
		RelatedTypes: flagTxPlanRelatedTypes,
		Status:       flagTxPlanStatus,
		Summary:      flagTxPlanSummary,
		BugId:        flagTxPlanBugId,
	}
	r, total, err := apiClient.TestxListPlanBugs(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanBugStats(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListPlanBugStatisticsRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, err := apiClient.TestxListPlanBugStatistics(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxPlanStories(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("plan-uid", flagTxPlanUid) {
		return nil
	}
	req := &model.TestxListPlanStoriesRequest{
		Namespace: flagTxNamespace,
		PlanUid:   flagTxPlanUid,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListPlanStories(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxPlanTemplates(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListPlanTemplatesRequest{
		Namespace: flagTxNamespace,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListPlanTemplates(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}
