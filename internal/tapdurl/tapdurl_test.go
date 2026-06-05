package tapdurl

import "testing"

func TestParse_DetailPath(t *testing.T) {
	cases := []struct {
		name           string
		url            string
		wantWS, wantT, wantID string
	}{
		{
			"story detail with tapd_fe prefix",
			"https://www.tapd.cn/tapd_fe/51081496/story/detail/1151081496001028684",
			"51081496", "story", "1151081496001028684",
		},
		{
			"bug detail",
			"https://www.tapd.cn/tapd_fe/123/bug/detail/456",
			"123", "bug", "456",
		},
		{
			"task detail",
			"https://www.tapd.cn/tapd_fe/123/task/detail/789",
			"123", "task", "789",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.url)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.WorkspaceID != tc.wantWS || got.EntityType != tc.wantT || got.EntityID != tc.wantID {
				t.Fatalf("got=%+v want=(%s %s %s)", got, tc.wantWS, tc.wantT, tc.wantID)
			}
		})
	}
}

// TestParse_ProngViewPath 是本轮的关键用例：用户给的真实 URL 必须能解析。
func TestParse_ProngViewPath(t *testing.T) {
	cases := []struct {
		name              string
		url               string
		wantWS, wantT, wantID string
	}{
		{
			"real user URL: stories view",
			"https://www.tapd.cn/20063271/prong/stories/view/1120063271004942331",
			"20063271", "story", "1120063271004942331",
		},
		{
			"bugs view",
			"https://www.tapd.cn/123/prong/bugs/view/4567",
			"123", "bug", "4567",
		},
		{
			"tasks view",
			"https://www.tapd.cn/123/prong/tasks/view/789",
			"123", "task", "789",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.url)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.WorkspaceID != tc.wantWS || got.EntityType != tc.wantT || got.EntityID != tc.wantID {
				t.Fatalf("got=%+v want=(%s %s %s)", got, tc.wantWS, tc.wantT, tc.wantID)
			}
		})
	}
}

func TestParse_DialogPreview(t *testing.T) {
	got, err := Parse("https://www.tapd.cn/tapd_fe/123/story/list?dialog_preview_id=story_999&foo=bar")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.WorkspaceID != "123" || got.EntityType != "story" || got.EntityID != "999" {
		t.Fatalf("got=%+v", got)
	}
}

func TestParse_WikiFragment(t *testing.T) {
	got, err := Parse("https://www.tapd.cn/123/markdown_wikis/show/#456")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.WorkspaceID != "123" || got.EntityType != "wiki" || got.EntityID != "456" {
		t.Fatalf("got=%+v", got)
	}
}

func TestParse_Errors(t *testing.T) {
	bads := []string{
		"https://www.tapd.cn/",                                       // 路径空
		"https://www.tapd.cn/tapd_fe/123/something/detail/456",       // 不支持的 type
		"https://www.tapd.cn/123/markdown_wikis/show/",               // wiki 缺 fragment
		"https://www.tapd.cn/123/prong/foobars/view/789",             // 不支持的 view 类型
		"not a url at all",                                           // 非 URL
	}
	for _, u := range bads {
		t.Run(u, func(t *testing.T) {
			if _, err := Parse(u); err == nil {
				t.Fatalf("expected error for %q", u)
			}
		})
	}
}
