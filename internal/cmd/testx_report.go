// Package cmd 中的 testx_report.go 实现了 testx report 子模块（4 API）
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagTxReportSearch    string
	flagTxReportStartAt   string
	flagTxReportEndAt     string
	flagTxReportCreators  []string
	flagTxReportPlanUids  []string
	flagTxReportWithAssoc bool
	flagTxReportTemplate  string
	flagTxReportSource    string
	flagTxReportSources   []string
)

// testxReportCmd 是 testx report 父命令
var testxReportCmd = &cobra.Command{
	Use:   "report",
	Short: "testx 报告模块（4 API）",
}

var testxReportListCmd = &cobra.Command{Use: "list", Short: "报告列表", RunE: runTestxReportList}
var testxReportGetCmd = &cobra.Command{Use: "get", Short: "报告详情", RunE: runTestxReportGet}
var testxReportDataCmd = &cobra.Command{Use: "get-data", Short: "报告详情数据", RunE: runTestxReportData}
var testxReportTemplatesCmd = &cobra.Command{Use: "list-templates", Short: "报告模板列表", RunE: runTestxReportTemplates}

func init() {
	testxAddPaginationFlags(testxReportListCmd)
	testxReportListCmd.Flags().StringVar(&flagTxReportSearch, "search", "", "搜索关键字")
	testxReportListCmd.Flags().StringVar(&flagTxReportStartAt, "start-at", "", "开始时间")
	testxReportListCmd.Flags().StringVar(&flagTxReportEndAt, "end-at", "", "结束时间")
	testxReportListCmd.Flags().StringSliceVar(&flagTxReportCreators, "creators", nil, "创建人（逗号分隔）")
	testxReportListCmd.Flags().StringSliceVar(&flagTxReportPlanUids, "plan-uids", nil, "计划 UID（逗号分隔）")
	testxReportListCmd.Flags().BoolVar(&flagTxReportWithAssoc, "with-associated", false, "包含关联")
	testxReportListCmd.Flags().StringVar(&flagTxReportTemplate, "template-uid", "", "模板 UID")
	testxReportListCmd.Flags().StringVar(&flagTxReportSource, "source", "", "来源")
	testxReportListCmd.Flags().StringSliceVar(&flagTxReportSources, "sources", nil, "来源列表（逗号分隔）")

	testxReportGetCmd.Flags().StringVar(&flagTxUid, "uid", "", "报告 UID（必填）")

	testxReportDataCmd.Flags().StringVar(&flagTxReportUid, "report-uid", "", "报告 UID（必填）")
	testxReportDataCmd.Flags().StringVar(&flagTxTemplateUid, "template-uid", "", "模板 UID（必填）")

	testxAddPaginationFlags(testxReportTemplatesCmd)

	testxReportCmd.AddCommand(testxReportListCmd, testxReportGetCmd, testxReportDataCmd, testxReportTemplatesCmd)
}

func runTestxReportList(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListReportsRequest{
		Namespace:      flagTxNamespace,
		PageInfo:       testxPageInfo(),
		Search:         flagTxReportSearch,
		StartAt:        flagTxReportStartAt,
		EndAt:          flagTxReportEndAt,
		Creators:       flagTxReportCreators,
		PlanUids:       flagTxReportPlanUids,
		WithAssociated: flagTxReportWithAssoc,
		TemplateUid:    flagTxReportTemplate,
		Source:         flagTxReportSource,
		Sources:        flagTxReportSources,
	}
	r, total, err := apiClient.TestxListReports(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxReportGet(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("uid", flagTxUid) {
		return nil
	}
	req := &model.TestxGetReportRequest{Namespace: flagTxNamespace, Uid: flagTxUid}
	r, err := apiClient.TestxGetReport(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxReportData(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() || !testxRequire("report-uid", flagTxReportUid) || !testxRequire("template-uid", flagTxTemplateUid) {
		return nil
	}
	req := &model.TestxGetReportDataRequest{
		Namespace:   flagTxNamespace,
		ReportUid:   flagTxReportUid,
		TemplateUid: flagTxTemplateUid,
	}
	r, err := apiClient.TestxGetReportData(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return output.PrintJSON(os.Stdout, r, !flagPretty)
}

func runTestxReportTemplates(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListReportTemplatesRequest{
		Namespace: flagTxNamespace,
		PageInfo:  testxPageInfo(),
	}
	r, total, err := apiClient.TestxListReportTemplates(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}
