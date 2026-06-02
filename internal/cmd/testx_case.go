// Package cmd 中的 testx_case.go 实现了 testx case 子模块（18 API）
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// case 子命令独有的 flags
var (
	flagTxCaseSearch       string
	flagTxCaseReverse      bool
	flagTxCaseExecOrdering string
	flagTxCaseRevSource    string
	flagTxCaseRevMainUid   string
	flagTxCaseRevSrcKind   string
	flagTxCaseRevSrcUid    string
	flagTxCaseRevIsLast    bool
	flagTxCaseBugStatus    string
	flagTxCaseBugPriority  string
	flagTxCaseBugHandler   string
	flagTxCaseBugName      string
)

// testxCaseCmd 是 testx case 父命令
var testxCaseCmd = &cobra.Command{
	Use:   "case",
	Short: "testx 用例模块（18 API）",
}

// ---- 子命令定义 ----

var testxCaseRepoCreateCmd = &cobra.Command{Use: "repo-create", Short: "创建用例仓库", RunE: runTestxCaseRepoCreate}
var testxCaseRepoUpdateCmd = &cobra.Command{Use: "repo-update", Short: "更新用例仓库", RunE: runTestxCaseRepoUpdate}
var testxCaseRepoGetCmd = &cobra.Command{Use: "repo-get", Short: "获取用例仓库", RunE: runTestxCaseRepoGet}
var testxCaseRepoListCmd = &cobra.Command{Use: "repo-list", Short: "获取用例仓库列表", RunE: runTestxCaseRepoList}

var testxCaseFolderCreateCmd = &cobra.Command{Use: "folder-create", Short: "创建用例目录", RunE: runTestxCaseFolderCreate}
var testxCaseFolderUpdateCmd = &cobra.Command{Use: "folder-update", Short: "更新用例目录", RunE: runTestxCaseFolderUpdate}

var testxCaseCreateCmd = &cobra.Command{Use: "create", Short: "创建用例", RunE: runTestxCaseCreate}
var testxCaseBatchCreateCmd = &cobra.Command{Use: "batch-create", Short: "批量创建用例", RunE: runTestxCaseBatchCreate}
var testxCaseUpdateCmd = &cobra.Command{Use: "update", Short: "更新用例", RunE: runTestxCaseUpdate}
var testxCaseBatchUpdateCmd = &cobra.Command{Use: "batch-update", Short: "批量更新用例", RunE: runTestxCaseBatchUpdate}
var testxCaseSearchCmd = &cobra.Command{Use: "search", Short: "搜索用例", RunE: runTestxCaseSearch}
var testxCaseHistoryCmd = &cobra.Command{Use: "list-history", Short: "用例变更历史", RunE: runTestxCaseHistory}
var testxCaseExecutionCmd = &cobra.Command{Use: "list-execution", Short: "用例执行记录", RunE: runTestxCaseExecution}
var testxCaseReviewCmd = &cobra.Command{Use: "list-review", Short: "用例评审记录", RunE: runTestxCaseReview}
var testxCaseBugListCmd = &cobra.Command{Use: "list-bug", Short: "用例关联缺陷列表", RunE: runTestxCaseBugList}
var testxCaseBugBindCmd = &cobra.Command{Use: "batch-bind-bug", Short: "批量关联 Bug", RunE: runTestxCaseBugBind}
var testxCaseBugUnbindCmd = &cobra.Command{Use: "batch-unbind-bug", Short: "批量解绑 Bug", RunE: runTestxCaseBugUnbind}
var testxCaseTemplateCmd = &cobra.Command{Use: "list-template", Short: "用例模板列表", RunE: runTestxCaseTemplate}

func init() {
	// 共享 flags 注册到每个子命令所需位置
	all := []*cobra.Command{
		testxCaseRepoCreateCmd, testxCaseRepoUpdateCmd, testxCaseRepoGetCmd, testxCaseRepoListCmd,
		testxCaseFolderCreateCmd, testxCaseFolderUpdateCmd,
		testxCaseCreateCmd, testxCaseBatchCreateCmd, testxCaseUpdateCmd, testxCaseBatchUpdateCmd,
		testxCaseSearchCmd, testxCaseHistoryCmd, testxCaseExecutionCmd, testxCaseReviewCmd,
		testxCaseBugListCmd, testxCaseBugBindCmd, testxCaseBugUnbindCmd, testxCaseTemplateCmd,
	}
	for _, c := range all {
		testxAddDataFlag(c)
	}

	// repo
	testxCaseRepoUpdateCmd.Flags().StringVar(&flagTxRepoUid, "repo-uid", "", "用例仓库 UID（必填）")
	testxCaseRepoGetCmd.Flags().StringVar(&flagTxRepoUid, "repo-uid", "", "用例仓库 UID（必填）")
	testxAddPaginationFlags(testxCaseRepoListCmd)
	testxCaseRepoListCmd.Flags().StringVar(&flagTxCaseSearch, "search", "", "按名称搜索")
	testxCaseRepoListCmd.Flags().BoolVar(&flagTxCaseReverse, "reverse", false, "反向排序")

	// repoUid + repoVersionUid 系列
	repoVersionFlags := func(c *cobra.Command) {
		c.Flags().StringVar(&flagTxRepoUid, "repo-uid", "", "用例仓库 UID（必填）")
		c.Flags().StringVar(&flagTxRepoVersionUid, "repo-version-uid", "", "用例仓库版本 UID（必填）")
	}
	for _, c := range []*cobra.Command{
		testxCaseFolderCreateCmd, testxCaseFolderUpdateCmd,
		testxCaseCreateCmd, testxCaseBatchCreateCmd, testxCaseUpdateCmd, testxCaseBatchUpdateCmd,
		testxCaseSearchCmd, testxCaseHistoryCmd, testxCaseExecutionCmd, testxCaseReviewCmd,
		testxCaseBugListCmd, testxCaseBugBindCmd, testxCaseBugUnbindCmd,
	} {
		repoVersionFlags(c)
	}

	testxCaseFolderUpdateCmd.Flags().StringVar(&flagTxFolderUid, "folder-uid", "", "目录 UID（必填）")

	// case-uid 系列
	for _, c := range []*cobra.Command{
		testxCaseUpdateCmd, testxCaseHistoryCmd, testxCaseExecutionCmd, testxCaseReviewCmd,
		testxCaseBugListCmd, testxCaseBugBindCmd, testxCaseBugUnbindCmd,
	} {
		c.Flags().StringVar(&flagTxCaseUid, "case-uid", "", "用例 UID（必填）")
	}

	// 分页
	for _, c := range []*cobra.Command{
		testxCaseSearchCmd, testxCaseHistoryCmd, testxCaseExecutionCmd, testxCaseReviewCmd, testxCaseBugListCmd,
	} {
		testxAddPaginationFlags(c)
	}

	testxCaseExecutionCmd.Flags().StringVar(&flagTxCaseExecOrdering, "ordering", "", "排序规则")
	testxCaseReviewCmd.Flags().StringVar(&flagTxCaseRevSource, "source", "", "评审来源")
	testxCaseReviewCmd.Flags().StringVar(&flagTxCaseRevMainUid, "main-uid", "", "评审主 UID")
	testxCaseReviewCmd.Flags().StringVar(&flagTxCaseRevSrcKind, "source-kind", "", "评审来源种类")
	testxCaseReviewCmd.Flags().StringVar(&flagTxCaseRevSrcUid, "source-uid", "", "评审来源 UID")
	testxCaseReviewCmd.Flags().BoolVar(&flagTxCaseRevIsLast, "is-last-review", false, "仅最新评审")
	testxCaseBugListCmd.Flags().StringVar(&flagTxCaseBugStatus, "status", "", "缺陷状态")
	testxCaseBugListCmd.Flags().StringVar(&flagTxCaseBugPriority, "priority", "", "缺陷优先级")
	testxCaseBugListCmd.Flags().StringVar(&flagTxCaseBugHandler, "handler", "", "处理人")
	testxCaseBugListCmd.Flags().StringVar(&flagTxCaseBugName, "name", "", "缺陷名称")

	testxCaseCmd.AddCommand(all...)
}

// ---- 运行函数 ----

func runTestxCaseRepoCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxCreateCaseRepoRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, err := apiClient.TestxCreateCaseRepo(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseRepoUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) {
		return nil
	}
	req := &model.TestxUpdateCaseRepoRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	r, err := apiClient.TestxUpdateCaseRepo(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseRepoGet(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) {
		return nil
	}
	req := &model.TestxGetCaseRepoRequest{Namespace: flagTxNamespace, RepoUid: flagTxRepoUid}
	r, err := apiClient.TestxGetCaseRepo(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseRepoList(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListCaseReposRequest{
		Namespace: flagTxNamespace,
		PageInfo:  testxPageInfo(),
		Search:    flagTxCaseSearch,
		Reverse:   flagTxCaseReverse,
	}
	r, total, err := apiClient.TestxListCaseRepos(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseFolderCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) {
		return nil
	}
	req := &model.TestxCreateCaseFolderRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	r, err := apiClient.TestxCreateCaseFolder(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseFolderUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("folder-uid", flagTxFolderUid) {
		return nil
	}
	req := &model.TestxUpdateCaseFolderRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	req.FolderUid = flagTxFolderUid
	r, err := apiClient.TestxUpdateCaseFolder(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) {
		return nil
	}
	req := &model.TestxCreateCaseRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	r, err := apiClient.TestxCreateCase(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseBatchCreate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) {
		return nil
	}
	req := &model.TestxBatchCreateCasesRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	r, err := apiClient.TestxBatchCreateCases(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxUpdateCaseRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	req.CaseUid = flagTxCaseUid
	r, err := apiClient.TestxUpdateCase(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxCaseBatchUpdate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) {
		return nil
	}
	req := &model.TestxBatchUpdateCasesRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	if err := apiClient.TestxBatchUpdateCases(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxCaseSearch(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) {
		return nil
	}
	req := &model.TestxSearchCasesRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	if pi := testxPageInfo(); pi != nil {
		req.PageInfo = pi
	}
	r, total, err := apiClient.TestxSearchCases(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseHistory(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxListCaseHistorysRequest{
		Namespace:      flagTxNamespace,
		RepoUid:        flagTxRepoUid,
		RepoVersionUid: flagTxRepoVersionUid,
		CaseUid:        flagTxCaseUid,
		PageInfo:       testxPageInfo(),
	}
	r, total, err := apiClient.TestxListCaseHistorys(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseExecution(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxListCaseExecutionsRequest{
		Namespace:      flagTxNamespace,
		RepoUid:        flagTxRepoUid,
		RepoVersionUid: flagTxRepoVersionUid,
		CaseUid:        flagTxCaseUid,
		PageInfo:       testxPageInfo(),
		Ordering:       flagTxCaseExecOrdering,
	}
	r, total, err := apiClient.TestxListCaseExecutions(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseReview(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxListCaseReviewsRequest{
		Namespace:      flagTxNamespace,
		RepoUid:        flagTxRepoUid,
		RepoVersionUid: flagTxRepoVersionUid,
		CaseUid:        flagTxCaseUid,
		PageInfo:       testxPageInfo(),
		Source:         flagTxCaseRevSource,
		MainUid:        flagTxCaseRevMainUid,
		SourceKind:     flagTxCaseRevSrcKind,
		SourceUid:      flagTxCaseRevSrcUid,
		IsLastReview:   flagTxCaseRevIsLast,
	}
	r, total, err := apiClient.TestxListCaseReviews(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseBugList(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxListCaseBugsRequest{
		Namespace:      flagTxNamespace,
		RepoUid:        flagTxRepoUid,
		RepoVersionUid: flagTxRepoVersionUid,
		CaseUid:        flagTxCaseUid,
		PageInfo:       testxPageInfo(),
		Status:         flagTxCaseBugStatus,
		Priority:       flagTxCaseBugPriority,
		Handler:        flagTxCaseBugHandler,
		Name:           flagTxCaseBugName,
	}
	r, total, err := apiClient.TestxListCaseBugs(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxCaseBugBind(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxBatchBindCaseBugRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	req.CaseUid = flagTxCaseUid
	if err := apiClient.TestxBatchBindCaseBug(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxCaseBugUnbind(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("repo-uid", flagTxRepoUid) || !testxRequire("repo-version-uid", flagTxRepoVersionUid) || !testxRequire("case-uid", flagTxCaseUid) {
		return nil
	}
	req := &model.TestxBatchUnbindCaseBugRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	req.RepoUid = flagTxRepoUid
	req.RepoVersionUid = flagTxRepoVersionUid
	req.CaseUid = flagTxCaseUid
	if err := apiClient.TestxBatchUnbindCaseBug(context.Background(), req); err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, map[string]bool{"success": true}, !flagPretty)
}

func runTestxCaseTemplate(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListCaseTemplatesRequest{Namespace: flagTxNamespace}
	r, err := apiClient.TestxListCaseTemplates(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}
