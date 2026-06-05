// Package cmd 中的 testx_design.go 实现了 testx design 子模块（3 API）
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagTxDesignUid  string
	flagTxDesignKind string
	flagTxDesignName string
)

// testxDesignCmd 是 testx design 父命令
var testxDesignCmd = &cobra.Command{
	Use:   "design",
	Short: "testx 测试设计模块（3 API）",
}

var testxDesignSearchCmd = &cobra.Command{Use: "search", Short: "搜索测试设计", RunE: runTestxDesignSearch}
var testxDesignStatsCmd = &cobra.Command{Use: "list-stat", Short: "测试设计统计", RunE: runTestxDesignStats}
var testxDesignLabelsCmd = &cobra.Command{Use: "list-labels", Short: "测试设计标签", RunE: runTestxDesignLabels}

func init() {
	testxAddDataFlag(testxDesignSearchCmd)
	testxAddDataFlag(testxDesignStatsCmd)

	testxDesignLabelsCmd.Flags().StringVar(&flagTxDesignUid, "design-uid", "", "测试设计 UID")
	testxDesignLabelsCmd.Flags().StringVar(&flagTxDesignKind, "kind", "", "标签种类")
	testxDesignLabelsCmd.Flags().StringVar(&flagTxDesignName, "name", "", "标签名称")

	testxDesignCmd.AddCommand(testxDesignSearchCmd, testxDesignStatsCmd, testxDesignLabelsCmd)
}

func runTestxDesignSearch(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxSearchDesignsRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, total, err := apiClient.TestxSearchDesigns(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}

func runTestxDesignStats(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListDesignStatsRequest{}
	if err := testxParseData(req); err != nil {
		return testxParamError(err)
	}
	req.Namespace = flagTxNamespace
	r, err := apiClient.TestxListDesignStats(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, 0, false)
}

func runTestxDesignLabels(cmd *cobra.Command, args []string) error {
	if !testxRequireNamespace() {
		return nil
	}
	req := &model.TestxListDesignLabelsRequest{
		Namespace: flagTxNamespace,
		DesignUid: flagTxDesignUid,
		Kind:      flagTxDesignKind,
		Name:      flagTxDesignName,
	}
	r, total, err := apiClient.TestxListDesignLabels(context.Background(), req)
	if err != nil {
		return testxAPIError(err)
	}
	return testxPrint(r, total, true)
}
