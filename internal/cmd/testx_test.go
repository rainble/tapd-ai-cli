// Package cmd 中的 testx_test.go 为 testx 子命令（case/plan/report/design）提供单元测试
package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// testxAPIHandler 返回 testx API 的 mock handler，根据实际 SDK 请求路径分发 testx 格式响应
//
// SDK 实际路径（按模块）:
//
//	case:   /api/testx/case/v1/namespaces/{ns}/repos[/{uid}]
//	        .../repos/{uid}/versions/{vid}/folders[/{fid}]
//	        .../versions/{vid}/cases[/{cid}]
//	        .../cases/batch, .../cases/search
//	        .../cases/{cid}/history, .../cases/{cid}/executions
//	        .../cases/{cid}/reviews, .../cases/{cid}/bugs[/bind|unbind]
//	        .../case-templates
//	plan:   /api/testx/plan/v1/namespaces/{ns}/folders[/{uid}]
//	        .../folders/children, .../folders/{fid}/plans-list
//	        .../plans[/{uid}], .../plans/{uid}/target-scope
//	        .../plans/{uid}/cases/batch-update, .../plans/batch-archive
//	        .../plans/{uid}/cases-search, .../plans/{uid}/histories
//	        .../plans/statistics, .../plans/{uid}/cases/bugs
//	        .../plans/{uid}/cases/{cid}/issues[/{iid}], .../plans/{uid}/cases/{cid}/events
//	        .../plans/{uid}/bugs, .../plans/bug-statistics
//	        .../plans/{uid}/stories, .../plan-templates
//	report: /api/testx/report/v1/namespaces/{ns}/reports[/{uid}]
//	        .../reports/{uid}/templates/{tid}/data, .../templates
//	design: /api/testx/design/v2/namespaces/{ns}/designs/search
//	        .../stat-list, .../labels
func testxAPIHandler() http.HandlerFunc {
	obj := `{"RequestId":"mock","Error":null,"Data":{"Uid":"uid-001","Name":"Test"},"TotalCount":0}`
	list := `{"RequestId":"mock","Error":null,"Data":[],"TotalCount":0}`
	null := `{"RequestId":"mock","Error":null,"Data":null,"TotalCount":0}`
	// SearchCases 返回 TestxSearchCasesResponse 结构
	searchCase := `{"RequestId":"mock","Error":null,"Data":{"Cases":[],"Total":0},"TotalCount":0}`
	// ListPlanCases 返回 TestxPlanCasesResult 结构
	planCases := `{"RequestId":"mock","Error":null,"Data":{"Cases":[],"Total":0},"TotalCount":0}`
	// FolderChildren 返回 TestxFolderChildrenResult 结构
	folderChildren := `{"RequestId":"mock","Error":null,"Data":{"Items":[]},"TotalCount":0}`

	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		switch {
		// ---- case 模块 ----
		case strings.Contains(p, "/api/testx/case/"):
			switch {
			// case-templates（无 repo/version 路径段）
			case strings.Contains(p, "/case-templates"):
				w.Write([]byte(list))
			// 用例搜索 POST .../cases/search
			case strings.HasSuffix(p, "/cases/search"):
				w.Write([]byte(searchCase))
			// 批量操作 .../cases/batch
			case strings.Contains(p, "/cases/batch"):
				w.Write([]byte(obj)) // batch-create 返回对象; batch-update PUT 由 SDK 不解析 Data
				if r.Method == http.MethodPut {
					w.Write(nil) // 已经写了 obj，SDK 对 batch-update 不解析 Data
				}
			// 用例历史 .../cases/{cid}/history
			case strings.Contains(p, "/history"):
				w.Write([]byte(list))
			// 用例执行 .../cases/{cid}/executions
			case strings.Contains(p, "/executions"):
				w.Write([]byte(list))
			// 用例评审 .../cases/{cid}/reviews
			case strings.Contains(p, "/reviews"):
				w.Write([]byte(list))
			// 用例 bug 绑定/解绑 .../cases/{cid}/bugs/bind 或 unbind
			case strings.Contains(p, "/bugs/bind"), strings.Contains(p, "/bugs/unbind"):
				w.Write([]byte(null))
			// 用例 bug 列表 .../cases/{cid}/bugs
			case strings.Contains(p, "/bugs"):
				w.Write([]byte(list))
			// 目录操作 .../folders
			case strings.Contains(p, "/folders"):
				w.Write([]byte(obj))
			// 仓库列表 GET .../repos（不含 repos/{uid}）
			case strings.HasSuffix(p, "/repos") && r.Method == http.MethodGet:
				w.Write([]byte(list))
			// 仓库 CRUD .../repos 或 .../repos/{uid}
			case strings.Contains(p, "/repos"):
				w.Write([]byte(obj))
			// 单个用例 CRUD .../cases 或 .../cases/{cid}
			default:
				w.Write([]byte(obj))
			}

		// ---- plan 模块 ----
		case strings.Contains(p, "/api/testx/plan/"):
			switch {
			// plan-templates（无 plans 路径段）
			case strings.Contains(p, "/plan-templates"):
				w.Write([]byte(list))
			// 目录子信息 .../folders/children
			case strings.Contains(p, "/folders/children"):
				w.Write([]byte(folderChildren))
			// 目录下计划列表 .../folders/{fid}/plans-list
			case strings.Contains(p, "/plans-list"):
				w.Write([]byte(list))
			// 目录操作 .../folders[/{uid}]
			case strings.Contains(p, "/folders"):
				w.Write([]byte(obj))
			// 计划用例事件 .../plans/{uid}/cases/{cid}/events
			case strings.Contains(p, "/events"):
				w.Write([]byte(list))
			// 计划用例缺陷（具体 issue）DELETE .../plans/{uid}/cases/{cid}/issues/{iid}
			case strings.Contains(p, "/issues/"):
				w.Write([]byte(null))
			// 计划用例缺陷列表 GET .../plans/{uid}/cases/{cid}/issues
			case strings.Contains(p, "/issues"):
				w.Write([]byte(list))
			// 批量更新计划用例 PUT .../plans/{uid}/cases/batch-update
			case strings.Contains(p, "/cases/batch-update"):
				w.Write([]byte(null))
			// 批量关联缺陷 POST .../plans/{uid}/cases/bugs
			case strings.Contains(p, "/cases/bugs"):
				w.Write([]byte(null))
			// 计划下用例搜索 POST .../plans/{uid}/cases-search
			case strings.Contains(p, "/cases-search"):
				w.Write([]byte(planCases))
			// 计划变更历史 .../plans/{uid}/histories
			case strings.Contains(p, "/histories"):
				w.Write([]byte(list))
			// 批量归档 POST .../plans/batch-archive
			case strings.Contains(p, "/batch-archive"):
				w.Write([]byte(null))
			// 计划 bug 统计 .../plans/bug-statistics
			case strings.Contains(p, "/bug-statistics"):
				w.Write([]byte(list))
			// 计划统计 .../plans/statistics
			case strings.Contains(p, "/statistics"):
				w.Write([]byte(list))
			// 计划关联需求 .../plans/{uid}/stories
			case strings.Contains(p, "/stories"):
				w.Write([]byte(list))
			// 计划关联缺陷列表 .../plans/{uid}/bugs
			case strings.Contains(p, "/bugs"):
				w.Write([]byte(list))
			// 更新计划范围 .../plans/{uid}/target-scope
			case strings.Contains(p, "/target-scope"):
				w.Write([]byte(obj))
			// 计划 CRUD .../plans 或 .../plans/{uid}
			default:
				w.Write([]byte(obj))
			}

		// ---- report 模块 ----
		case strings.Contains(p, "/api/testx/report/"):
			switch {
			// 报告模板 .../templates（不在 reports/ 子路径下）
			case strings.Contains(p, "/templates") && !strings.Contains(p, "/reports/"):
				w.Write([]byte(list))
			// 报告数据 .../reports/{uid}/templates/{tid}/data
			case strings.Contains(p, "/data"):
				w.Write([]byte(obj))
			// 报告列表 GET .../reports 或 报告详情 GET .../reports/{uid}
			// SDK 对两者的 Data 都按 []model.TestxReport 解析
			default:
				w.Write([]byte(list))
			}

		// ---- design 模块 ----
		case strings.Contains(p, "/api/testx/design/"):
			switch {
			// 测试设计标签 .../labels
			case strings.Contains(p, "/labels"):
				w.Write([]byte(list))
			// 测试设计统计 .../stat-list
			case strings.Contains(p, "/stat-list"):
				w.Write([]byte(list))
			// 测试设计搜索 .../designs/search
			default:
				w.Write([]byte(list))
			}

		default:
			w.Write([]byte(obj))
		}
	}
}

// ===================== 辅助函数测试 =====================

func TestTestxParseData_Valid(t *testing.T) {
	resetFlags()
	flagTxData = `{"Name":"hello"}`
	type target struct {
		Name string `json:"Name"`
	}
	var v target
	if err := testxParseData(&v); err != nil {
		t.Fatalf("testxParseData failed: %v", err)
	}
	if v.Name != "hello" {
		t.Errorf("Name = %q, want %q", v.Name, "hello")
	}
}

func TestTestxParseData_Empty(t *testing.T) {
	resetFlags()
	flagTxData = ""
	var v struct{}
	if err := testxParseData(&v); err != nil {
		t.Fatalf("expected nil error for empty data, got: %v", err)
	}
}

func TestTestxParseData_Invalid(t *testing.T) {
	resetFlags()
	flagTxData = "not-json"
	var v struct{}
	if err := testxParseData(&v); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTestxPageInfo_Zero(t *testing.T) {
	resetFlags()
	flagTxOffset = 0
	flagTxLimit = 0
	if pi := testxPageInfo(); pi != nil {
		t.Errorf("expected nil, got %+v", pi)
	}
}

func TestTestxPageInfo_NonZero(t *testing.T) {
	resetFlags()
	flagTxOffset = 10
	flagTxLimit = 20
	pi := testxPageInfo()
	if pi == nil {
		t.Fatal("expected non-nil PageInfo")
	}
	if pi.Offset != 10 || pi.Limit != 20 {
		t.Errorf("PageInfo = %+v, want Offset=10 Limit=20", pi)
	}
}

func TestTestxPrint_WithTotal(t *testing.T) {
	resetFlags()
	restore, reader := captureStdout(t)
	err := testxPrint([]string{"a"}, 5, true)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("testxPrint failed: %v", err)
	}
}

func TestTestxPrint_WithoutTotal(t *testing.T) {
	resetFlags()
	restore, reader := captureStdout(t)
	err := testxPrint(map[string]string{"k": "v"}, 0, false)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("testxPrint failed: %v", err)
	}
}

// ===================== Case 子命令测试 =====================

func TestRunTestxCaseRepoCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxCaseRepoCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseRepoCreate failed: %v", err)
	}
}

func TestRunTestxCaseRepoUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseRepoUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseRepoUpdate failed: %v", err)
	}
}

func TestRunTestxCaseRepoGet(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseRepoGet(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseRepoGet failed: %v", err)
	}
}

func TestRunTestxCaseRepoList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxCaseRepoList(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseRepoList failed: %v", err)
	}
}

func TestRunTestxCaseFolderCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseFolderCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseFolderCreate failed: %v", err)
	}
}

func TestRunTestxCaseFolderUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxFolderUid = "folder-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseFolderUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseFolderUpdate failed: %v", err)
	}
}

func TestRunTestxCaseCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseCreate failed: %v", err)
	}
}

func TestRunTestxCaseBatchCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseBatchCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseBatchCreate failed: %v", err)
	}
}

func TestRunTestxCaseUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseUpdate failed: %v", err)
	}
}

func TestRunTestxCaseBatchUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseBatchUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseBatchUpdate failed: %v", err)
	}
}

func TestRunTestxCaseSearch(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseSearch(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseSearch failed: %v", err)
	}
}

func TestRunTestxCaseHistory(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseHistory(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseHistory failed: %v", err)
	}
}

func TestRunTestxCaseExecution(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseExecution(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseExecution failed: %v", err)
	}
}

func TestRunTestxCaseReview(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseReview(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseReview failed: %v", err)
	}
}

func TestRunTestxCaseBugList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseBugList(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseBugList failed: %v", err)
	}
}

func TestRunTestxCaseBugBind(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseBugBind(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseBugBind failed: %v", err)
	}
}

func TestRunTestxCaseBugUnbind(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxRepoUid = "repo-001"
	flagTxRepoVersionUid = "ver-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxCaseBugUnbind(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseBugUnbind failed: %v", err)
	}
}

func TestRunTestxCaseTemplate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxCaseTemplate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxCaseTemplate failed: %v", err)
	}
}

// ===================== Plan 子命令测试 =====================

func TestRunTestxPlanFolderCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanFolderCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanFolderCreate failed: %v", err)
	}
}

func TestRunTestxPlanFolderUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxFolderUid = "folder-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanFolderUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanFolderUpdate failed: %v", err)
	}
}

func TestRunTestxPlanFolderChildren(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "folder-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanFolderChildren(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanFolderChildren failed: %v", err)
	}
}

func TestRunTestxPlanGet(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanGet(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanGet failed: %v", err)
	}
}

func TestRunTestxPlanCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanCreate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanCreate failed: %v", err)
	}
}

func TestRunTestxPlanUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanUpdate(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanUpdate failed: %v", err)
	}
}

func TestRunTestxPlanUpdateScope(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanUpdateScope(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanUpdateScope failed: %v", err)
	}
}

func TestRunTestxPlanBatchUpdateCase(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanBatchUpdateCase(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBatchUpdateCase failed: %v", err)
	}
}

func TestRunTestxPlanBatchArchive(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanBatchArchive(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBatchArchive failed: %v", err)
	}
}

func TestRunTestxPlanList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxFolderUid = "folder-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanList(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanList failed: %v", err)
	}
}

func TestRunTestxPlanCases(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanCases(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanCases failed: %v", err)
	}
}

func TestRunTestxPlanHistory(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanHistory(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanHistory failed: %v", err)
	}
}

func TestRunTestxPlanStats(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanStats(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanStats failed: %v", err)
	}
}

func TestRunTestxPlanBugBind(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanBugBind(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBugBind failed: %v", err)
	}
}

func TestRunTestxPlanBugUnbind(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"
	flagTxCaseUid = "case-001"
	flagTxIssueUid = "issue-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanBugUnbind(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBugUnbind failed: %v", err)
	}
}

func TestRunTestxPlanCaseIssues(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"
	flagTxCaseUid = "case-001"
	flagTxPlanIssueType = "bug"

	restore, reader := captureStdout(t)
	err := runTestxPlanCaseIssues(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanCaseIssues failed: %v", err)
	}
}

func TestRunTestxPlanCaseEvents(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"
	flagTxCaseUid = "case-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanCaseEvents(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanCaseEvents failed: %v", err)
	}
}

func TestRunTestxPlanBugs(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanBugs(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBugs failed: %v", err)
	}
}

func TestRunTestxPlanBugStats(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanBugStats(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanBugStats failed: %v", err)
	}
}

func TestRunTestxPlanStories(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxPlanUid = "plan-001"

	restore, reader := captureStdout(t)
	err := runTestxPlanStories(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanStories failed: %v", err)
	}
}

func TestRunTestxPlanTemplates(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxPlanTemplates(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxPlanTemplates failed: %v", err)
	}
}

// ===================== Report 子命令测试 =====================

func TestRunTestxReportList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxReportList(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxReportList failed: %v", err)
	}
}

func TestRunTestxReportGet(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxUid = "report-001"

	restore, reader := captureStdout(t)
	err := runTestxReportGet(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxReportGet failed: %v", err)
	}
}

func TestRunTestxReportData(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"
	flagTxReportUid = "report-001"
	flagTxTemplateUid = "tmpl-001"

	restore, reader := captureStdout(t)
	err := runTestxReportData(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxReportData failed: %v", err)
	}
}

func TestRunTestxReportTemplates(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxReportTemplates(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxReportTemplates failed: %v", err)
	}
}

// ===================== Design 子命令测试 =====================

func TestRunTestxDesignSearch(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxDesignSearch(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxDesignSearch failed: %v", err)
	}
}

func TestRunTestxDesignStats(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxDesignStats(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxDesignStats failed: %v", err)
	}
}

func TestRunTestxDesignLabels(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, testxAPIHandler())
	defer cleanup()
	flagTxNamespace = "ns1"

	restore, reader := captureStdout(t)
	err := runTestxDesignLabels(nil, nil)
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runTestxDesignLabels failed: %v", err)
	}
}

// ===================== Flags 注册验证 =====================

func TestTestxFlagsRegistered(t *testing.T) {
	if testxCmd.PersistentFlags().Lookup("namespace") == nil {
		t.Error("testx should register --namespace")
	}
}

func TestTestxCaseFlagsRegistered(t *testing.T) {
	cases := []struct {
		cmd  string
		flag string
	}{
		{"repo-list", "search"},
		{"repo-list", "reverse"},
		{"repo-list", "offset"},
		{"repo-list", "limit"},
		{"repo-update", "repo-uid"},
		{"repo-get", "repo-uid"},
		{"folder-create", "repo-uid"},
		{"folder-create", "repo-version-uid"},
		{"folder-update", "folder-uid"},
		{"update", "case-uid"},
		{"search", "offset"},
		{"list-execution", "ordering"},
		{"list-review", "source"},
		{"list-bug", "status"},
		{"list-bug", "priority"},
		{"list-bug", "handler"},
		{"list-bug", "name"},
	}
	for _, c := range testxCaseCmd.Commands() {
		for _, tc := range cases {
			if c.Use == tc.cmd {
				if c.Flags().Lookup(tc.flag) == nil {
					t.Errorf("testx case %s should register --%s", tc.cmd, tc.flag)
				}
			}
		}
	}
}

func TestTestxPlanFlagsRegistered(t *testing.T) {
	checks := map[string][]string{
		"folder-update":     {"folder-uid"},
		"folder-children":   {"uid", "with-descendant", "with-ancestor", "name"},
		"get":               {"uid", "with-statistic", "with-detail"},
		"update":            {"uid"},
		"batch-update-case": {"plan-uid"},
		"list":              {"folder-uid"},
		"list-cases":        {"uid"},
		"list-history":      {"plan-uid", "offset"},
		"bind-bug":          {"plan-uid"},
		"unbind-bug":        {"plan-uid", "case-uid", "issue-uid"},
		"list-case-issues":  {"plan-uid", "case-uid", "issue-type"},
		"list-bugs":         {"plan-uid", "related-types", "status", "summary", "bug-id"},
		"list-stories":      {"plan-uid", "offset"},
		"list-templates":    {"offset"},
	}
	for _, c := range testxPlanCmd.Commands() {
		if flags, ok := checks[c.Use]; ok {
			for _, f := range flags {
				if c.Flags().Lookup(f) == nil {
					t.Errorf("testx plan %s should register --%s", c.Use, f)
				}
			}
		}
	}
}

func TestTestxReportFlagsRegistered(t *testing.T) {
	for _, f := range []string{"search", "start-at", "end-at", "creators", "plan-uids", "with-associated", "template-uid", "source", "sources", "offset"} {
		if testxReportListCmd.Flags().Lookup(f) == nil {
			t.Errorf("testx report list should register --%s", f)
		}
	}
	if testxReportGetCmd.Flags().Lookup("uid") == nil {
		t.Error("testx report get should register --uid")
	}
	if testxReportDataCmd.Flags().Lookup("report-uid") == nil {
		t.Error("testx report get-data should register --report-uid")
	}
	if testxReportDataCmd.Flags().Lookup("template-uid") == nil {
		t.Error("testx report get-data should register --template-uid")
	}
}

func TestTestxDesignFlagsRegistered(t *testing.T) {
	for _, f := range []string{"design-uid", "kind", "name"} {
		if testxDesignLabelsCmd.Flags().Lookup(f) == nil {
			t.Errorf("testx design list-labels should register --%s", f)
		}
	}
	if testxDesignSearchCmd.Flags().Lookup("data") == nil {
		t.Error("testx design search should register --data")
	}
	if testxDesignStatsCmd.Flags().Lookup("data") == nil {
		t.Error("testx design list-stat should register --data")
	}
}
