// Package cmd 中的 testx.go 实现了 TAPD testx 模块（用例/计划/报告/设计）的根命令与共享辅助函数
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// testx 子命令共享的全局标志
var (
	flagTxNamespace      string
	flagTxRepoUid        string
	flagTxRepoVersionUid string
	flagTxFolderUid      string
	flagTxCaseUid        string
	flagTxPlanUid        string
	flagTxIssueUid       string
	flagTxUid            string
	flagTxReportUid      string
	flagTxTemplateUid    string
	flagTxData           string
	flagTxOffset         uint32
	flagTxLimit          uint32
)

// testxCmd 是 testx 父命令
var testxCmd = &cobra.Command{
	Use:   "testx",
	Short: "TAPD testx 模块（用例/计划/报告/设计）",
	Long:  "TAPD testx 模块的命令集合，覆盖 case/plan/report/design 四个子模块共 46 个 API。",
}

func init() {
	testxCmd.PersistentFlags().StringVar(&flagTxNamespace, "namespace", "", "testx 命名空间（必填）")

	testxCmd.AddCommand(testxCaseCmd, testxPlanCmd, testxReportCmd, testxDesignCmd)
	rootCmd.AddCommand(testxCmd)
}

// testxRequireNamespace 校验 --namespace 标志非空
func testxRequireNamespace() bool {
	if flagTxNamespace == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--namespace is required", "")
		os.Exit(output.ExitParamError)
		return false
	}
	return true
}

// testxRequire 通用必填校验
func testxRequire(name, value string) bool {
	if value == "" {
		output.PrintError(os.Stderr, "missing_parameter", fmt.Sprintf("--%s is required", name), "")
		os.Exit(output.ExitParamError)
		return false
	}
	return true
}

// testxParseData 将 --data JSON 反序列化到目标请求结构体的 body 字段；空字符串则跳过
func testxParseData(target any) error {
	if flagTxData == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(flagTxData), target); err != nil {
		return fmt.Errorf("invalid JSON for --data: %w", err)
	}
	return nil
}

// testxPageInfo 当 --offset 或 --limit 至少一个被设置时，构造 PageInfo
func testxPageInfo() *model.TestxPageInfo {
	if flagTxOffset == 0 && flagTxLimit == 0 {
		return nil
	}
	return &model.TestxPageInfo{Offset: flagTxOffset, Limit: flagTxLimit}
}

// testxAPIError 统一 API 错误处理
func testxAPIError(err error) error {
	output.PrintError(os.Stderr, "api_error", err.Error(), "")
	os.Exit(output.ExitAPIError)
	return nil
}

// testxParamError 统一参数错误处理
func testxParamError(err error) error {
	output.PrintError(os.Stderr, "invalid_parameter", err.Error(), "")
	os.Exit(output.ExitParamError)
	return nil
}

// testxPrint 通用结果输出（带 total 时打印 list+total）
func testxPrint(data any, total int, withTotal bool) error {
	if withTotal {
		return output.PrintJSON(os.Stdout, map[string]any{"items": data, "total": total}, !flagPretty)
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

// testxAddPaginationFlags 为命令添加分页 flags
func testxAddPaginationFlags(c *cobra.Command) {
	c.Flags().Uint32Var(&flagTxOffset, "offset", 0, "分页偏移")
	c.Flags().Uint32Var(&flagTxLimit, "limit", 0, "分页大小")
}

// testxAddDataFlag 为命令添加 --data flag
func testxAddDataFlag(c *cobra.Command) {
	c.Flags().StringVar(&flagTxData, "data", "", "请求体 JSON（用于填充 body 字段）")
}
