package xiaohongshu

import (
	"errors"
	"testing"

	xhserrors "github.com/xpzouying/xiaohongshu-mcp/errors"
)

// 这组测试锁的是"三种失败原因必须可区分"这条不变量。
// 原实现把它们全压成「没有捕获到 feeds 数据」，使用者无法判断
// 该重新登录、换号，还是换关键词。
func TestSnapshotProblem(t *testing.T) {
	cases := []struct {
		name string
		snap PageSnapshot
		want error
	}{
		{
			name: "正常已登录页面",
			snap: PageSnapshot{
				URL:           "https://www.xiaohongshu.com/search_result?keyword=x",
				Title:         "小红书 - 你的生活指南",
				HasSearchNode: true,
				HasUserNode:   true,
			},
			want: nil,
		},
		{
			name: "URL 落到登录页",
			snap: PageSnapshot{URL: "https://www.xiaohongshu.com/login?redirect=x"},
			want: xhserrors.ErrNotLoggedIn,
		},
		{
			name: "页面出现登录 UI",
			snap: PageSnapshot{
				URL:        "https://www.xiaohongshu.com/search_result?keyword=x",
				HasLoginUI: true,
			},
			want: xhserrors.ErrNotLoggedIn,
		},
		{
			name: "平台明确返回账号异常",
			snap: PageSnapshot{
				URL:     "https://www.xiaohongshu.com/search_result?keyword=x",
				RawCode: "300011",
			},
			want: xhserrors.ErrAccountAbnormal,
		},
		{
			name: "错误信息里带 Account abnormal 也算账号异常",
			snap: PageSnapshot{
				URL:    "https://www.xiaohongshu.com/search_result?keyword=x",
				RawMsg: "Account abnormal. Switch account and retry.",
			},
			want: xhserrors.ErrAccountAbnormal,
		},
		{
			name: "笔记被拦截（300031）不能被误判成验证码",
			snap: PageSnapshot{
				URL: "https://www.xiaohongshu.com/404?source=/404/sec_alaRUWwP?redirectPath=" +
					"https%3A%2F%2Fwww.xiaohongshu.com%2Fexplore%2Fabc%3Fxsec_token%3Dxyz" +
					"&error_code=300031&error_msg=%E5%BD%93%E5%89%8D%E7%AC%94%E8%AE%B0%E6%9A%82%E6%97%B6%E6%97%A0%E6%B3%95%E6%B5%8F%E8%A7%88&uuid=x&verifyMsg=",
				Title: "小红书 - 你访问的页面不见了",
			},
			want: xhserrors.ErrNoteBlocked,
		},
		{
			name: "安全验证页必须被识别（实测踩过的坑）",
			snap: PageSnapshot{
				URL:   "https://www.xiaohongshu.com/website-login/captcha?verifyType=124&verifyBiz=461",
				Title: "安全验证",
			},
			want: xhserrors.ErrCaptchaRequired,
		},
		{
			name: "标题含安全验证也算",
			snap: PageSnapshot{
				URL:   "https://www.xiaohongshu.com/unknown",
				Title: "安全验证",
			},
			want: xhserrors.ErrCaptchaRequired,
		},
		{
			name: "验证码优先级高于未登录（它更具体）",
			snap: PageSnapshot{
				URL:        "https://www.xiaohongshu.com/website-login/captcha",
				HasLoginUI: true,
			},
			want: xhserrors.ErrCaptchaRequired,
		},
		{
			name: "登录页优先于账号异常被识别（先重登再判断）",
			snap: PageSnapshot{
				URL:     "https://www.xiaohongshu.com/login",
				RawCode: "300011",
			},
			want: xhserrors.ErrNotLoggedIn,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := snapshotProblem(tc.snap)
			if !errors.Is(got, tc.want) && !(got == nil && tc.want == nil) {
				t.Fatalf("snapshotProblem() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Describe 必须把人需要的信息都放进去，否则报错仍然无法行动。
func TestSnapshotDescribe(t *testing.T) {
	snap := PageSnapshot{
		URL:        "https://www.xiaohongshu.com/search_result?keyword=x",
		Title:      "小红书",
		HasLoginUI: true,
	}
	got := snap.Describe()
	for _, want := range []string{"状态=", "url=", "标题=", "登录UI=true"} {
		if !containsStr(got, want) {
			t.Fatalf("Describe() = %q, 缺少 %q", got, want)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
