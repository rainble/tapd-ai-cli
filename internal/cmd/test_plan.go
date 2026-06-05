// Package cmd 中的 test_plan.go 实现了测试计划管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagTestPlanID string
)

// testPlanCmd 是 test-plan 父命令
var testPlanCmd = &cobra.Command{
	Use:   "test-plan",
	Short: "测试计划管理",
}

var testPlanListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询测试计划列表",
	RunE:  runTestPlanList,
}

var testPlanCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建测试计划",
	RunE:  runTestPlanCreate,
}

var testPlanUpdateCmd = &cobra.Command{
	Use:   "update <test_plan_id>",
	Short: "更新测试计划",
	Args:  cobra.ExactArgs(1),
	RunE:  runTestPlanUpdate,
}

var testPlanCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询测试计划数量",
	RunE:  runTestPlanCount,
}

var testPlanProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "查询测试计划执行进度",
	RunE:  runTestPlanProgress,
}

var testPlanTCasesCmd = &cobra.Command{
	Use:   "tcases",
	Short: "查询测试计划关联的测试用例",
	RunE:  runTestPlanTCases,
}

var testPlanBugsCmd = &cobra.Command{
	Use:   "bugs",
	Short: "查询测试计划关联的缺陷",
	RunE:  runTestPlanBugs,
}

func init() {
	testPlanListCmd.Flags().StringVar(&flagStatus, "status", "", "按状态筛选")
	testPlanListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选")
	testPlanListCmd.Flags().IntVar(&flagLimit, "limit", 10, "返回数量限制")
	testPlanListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	testPlanListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	testPlanCreateCmd.Flags().StringVar(&flagName, "name", "", "测试计划标题（必需）")
	testPlanCreateCmd.Flags().StringVar(&flagIterationID, "iteration-id", "", "关联迭代 ID")
	testPlanCreateCmd.Flags().StringVar(&flagOwner, "owner", "", "负责人")
	testPlanCreateCmd.Flags().StringVar(&flagDescription, "description", "", "详细描述")
	testPlanCreateCmd.Flags().StringVar(&flagBegin, "start-date", "", "预计开始日期")
	testPlanCreateCmd.Flags().StringVar(&flagDue, "end-date", "", "预计结束日期")

	testPlanUpdateCmd.Flags().StringVar(&flagName, "name", "", "新标题")
	testPlanUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "新状态")
	testPlanUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "新负责人")
	testPlanUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "新描述")

	testPlanCountCmd.Flags().StringVar(&flagStatus, "status", "", "按状态筛选")

	testPlanProgressCmd.Flags().StringVar(&flagTestPlanID, "id", "", "测试计划 ID（必需）")
	testPlanTCasesCmd.Flags().StringVar(&flagTestPlanID, "id", "", "测试计划 ID（必需）")
	testPlanTCasesCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	testPlanTCasesCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	testPlanBugsCmd.Flags().StringVar(&flagTestPlanID, "id", "", "测试计划 ID（必需）")

	testPlanCmd.AddCommand(testPlanListCmd, testPlanCreateCmd, testPlanUpdateCmd,
		testPlanCountCmd, testPlanProgressCmd, testPlanTCasesCmd, testPlanBugsCmd)
	rootCmd.AddCommand(testPlanCmd)
}

func runTestPlanList(cmd *cobra.Command, args []string) error {
	req := &model.ListTestPlansRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Status:      flagStatus,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	plans, err := listWithFilters[model.TestPlan](cmdContext(cmd), apiClient, "/test_plans", req.ToParams(), flagFilter, "TestPlan")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	total, _ := apiClient.CountTestPlans(context.Background(), req)
	resp := &model.ListResponse{
		Items:   plans,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runTestPlanCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--name is required",
			"Usage: tapd test-plan create --name <name>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.CreateTestPlanRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Description: flagDescription,
		Owner:       flagOwner,
		StartDate:   flagBegin,
		EndDate:     flagDue,
		IterationID: flagIterationID,
	}
	result, err := apiClient.CreateTestPlan(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runTestPlanUpdate(cmd *cobra.Command, args []string) error {
	req := &model.UpdateTestPlanRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Name:        flagName,
		Status:      flagStatus,
		Owner:       flagOwner,
		Description: flagDescription,
	}
	result, err := apiClient.UpdateTestPlan(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runTestPlanCount(cmd *cobra.Command, args []string) error {
	req := &model.ListTestPlansRequest{
		WorkspaceID: flagWorkspaceID,
		Status:      flagStatus,
	}
	count, err := apiClient.CountTestPlans(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}

func runTestPlanProgress(cmd *cobra.Command, args []string) error {
	if flagTestPlanID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--id is required",
			"Usage: tapd test-plan progress --id <plan_id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetTestPlanProgressRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          flagTestPlanID,
	}
	progress, err := apiClient.GetTestPlanProgress(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, progress, !flagPretty)
}

func runTestPlanTCases(cmd *cobra.Command, args []string) error {
	if flagTestPlanID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--id is required",
			"Usage: tapd test-plan tcases --id <plan_id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.TestPlanTCasesRequest{
		WorkspaceID: flagWorkspaceID,
		TestPlanID:  flagTestPlanID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	relations, err := apiClient.ListTestPlanTCaseRelations(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, relations, !flagPretty)
}

func runTestPlanBugs(cmd *cobra.Command, args []string) error {
	if flagTestPlanID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--id is required",
			"Usage: tapd test-plan bugs --id <plan_id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.TestPlanIDRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          flagTestPlanID,
	}
	data, err := apiClient.GetTestPlanBugs(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}
