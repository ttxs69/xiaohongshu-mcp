package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"

	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// PageSnapshot 是页面此刻的可诊断状态。
//
// 存在的理由：原实现在取不到数据时只返回「没有捕获到 feeds 数据 /
// 没有捕获到 feed 详情数据」，而三种完全不同的情况都会落到这里：
//  1. 未登录 / cookie 失效（页面跳到登录墙）
//  2. 账号被风控（平台返回 Account abnormal）
//  3. 真的没有结果
//
// 使用者拿到同一句话，只能盲目重试。本文件让失败原因可分辨。
type PageSnapshot struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	HasLoginUI    bool   `json:"has_login_ui"`
	HasSearchNode bool   `json:"has_search_node"`
	HasUserNode   bool   `json:"has_user_node"`
	RawCode       string `json:"raw_code,omitempty"`
	RawMsg        string `json:"raw_msg,omitempty"`
}

// diagEvalTimeout 是诊断类 Eval 的上限。
// 诊断必须"廉价且快"，否则它会变成新的阻塞源。
const diagEvalTimeout = 1500 * time.Millisecond

const snapshotJS = `() => {
	const s = window.__INITIAL_STATE__;
	const q = sel => { try { return !!document.querySelector(sel); } catch (e) { return false; } };
	const code = s && (s.errorCode || s.code);
	const msg = s && (s.errorMsg || s.message);
	return JSON.stringify({
		url: location.href,
		title: document.title,
		has_login_ui: q('.login-container') || q('.qrcode-img') || q('#login-btn') || q('.sign-in-container'),
		has_search_node: !!(s && s.search),
		has_user_node: !!(s && s.user),
		raw_code: code ? String(code) : '',
		raw_msg: msg ? String(msg) : ''
	});
}`

// Snapshot 读取页面当前状态。取不到也不报错——诊断信息本身不该再制造失败。
//
// 关键：必须用短超时。page 通常带着 60s 的 deadline，
// 直接 page.Eval 在页面忙/导航中会阻塞到那个 deadline，
// 结果是「诊断轮询」每 60 秒才转一圈，等于没有轮询。
// 实测（2026-10-21）：不加短超时时，安全验证页要 61 秒才被识别出来。
func Snapshot(page *rod.Page) PageSnapshot {
	var snap PageSnapshot
	res, err := page.Timeout(diagEvalTimeout).Eval(snapshotJS)
	if err != nil {
		if info, ierr := page.Info(); ierr == nil {
			snap.URL = info.URL
			snap.Title = info.Title
		}
		return snap
	}
	if raw := res.Value.Str(); raw != "" {
		_ = json.Unmarshal([]byte(raw), &snap)
	}
	return snap
}

// LoginProblem 判断页面是否处于"未登录 / 登录失效 / 账号异常"。
// 返回 nil 表示看不出登录类问题。
func LoginProblem(page *rod.Page) error {
	return snapshotProblem(Snapshot(page))
}

func snapshotProblem(snap PageSnapshot) error {
	lower := strings.ToLower(snap.URL)

	// 判定顺序很重要，且不能只看 URL 关键词。
	//
	// 实测踩过的坑（2026-10-21）：
	//   笔记被拦截时 URL 是
	//     /404?source=/404/sec_alaRUWwP?...&error_code=300031&error_msg=当前笔记暂时无法浏览
	//   其中 redirectPath 里含 "verifyMsg"、路径含 "sec_"，很容易被
	//   「URL 含 captcha/verify」这种粗判误认成验证码。
	//   而它其实是**笔记被拦截**，两者处置完全不同：
	//   验证码要人工过；笔记被拦截只能换一条笔记。

	// 1) 笔记被拦截：URL 里带 error_code=300031 或 source=/404/sec_
	if strings.Contains(lower, "error_code=300031") ||
		strings.Contains(lower, "/404/sec_") {
		return errors.ErrNoteBlocked
	}

	// 2) 登录墙：先于账号风控判定。
	//    实测 cookie 失效时平台可能同时给 300011，
	//    正确动作是**先重新登录**，重登后仍 300011 才是真的被风控。
	//    反过来（先认定风控去换号）可能白换一个号。
	if strings.Contains(lower, "/login") {
		return errors.ErrNotLoggedIn
	}

	// 3) 平台明确的风控码
	if snap.RawCode == "300011" || strings.Contains(strings.ToLower(snap.RawMsg), "account abnormal") {
		return errors.ErrAccountAbnormal
	}

	// 4) 验证码 / 安全验证：路径里真的出现 captcha 段，或标题就是「安全验证」。
	//    只看 path 段与标题，不拿整串 query 做 includes——
	//    redirectPath 里出现 "verifyMsg" 并不代表当前页是验证码页。
	if strings.Contains(lower, "/website-login/captcha") ||
		strings.Contains(lower, "/captcha?") ||
		strings.Contains(snap.Title, "安全验证") ||
		strings.Contains(snap.Title, "验证码") {
		return errors.ErrCaptchaRequired
	}
	if snap.HasLoginUI {
		return errors.ErrNotLoggedIn
	}
	return nil
}

// Describe 把快照压成一句可行动的话，用于拼进错误信息。
func (s PageSnapshot) Describe() string {
	state := "已登录"
	switch {
	case snapshotProblem(s) == errors.ErrNoteBlocked:
		state = "笔记被拦截"
	case snapshotProblem(s) == errors.ErrCaptchaRequired:
		state = "要求安全验证"
	case snapshotProblem(s) == errors.ErrAccountAbnormal:
		state = "账号异常"
	case snapshotProblem(s) == errors.ErrNotLoggedIn:
		state = "未登录/登录失效"
	case s.HasLoginUI:
		state = "疑似登录墙"
	}
	return fmt.Sprintf("页面诊断: 状态=%s url=%s 标题=%q 登录UI=%t search节点=%t user节点=%t code=%s",
		state, s.URL, s.Title, s.HasLoginUI, s.HasSearchNode, s.HasUserNode, s.RawCode)
}

// EmptyResultError 为空结果补上现场信息。
// 关键点：如果现场其实是登录问题，就返回那个更具体的错误，而不是"没有数据"。
func EmptyResultError(page *rod.Page, base error) error {
	snap := Snapshot(page)
	if p := snapshotProblem(snap); p != nil {
		return fmt.Errorf("%w（%s）", p, snap.Describe())
	}
	return fmt.Errorf("%w（%s）", base, snap.Describe())
}

// WaitSearchReady 等待搜索页就绪，但**同时监视登录墙**。
//
// 为什么不能用 page.MustWait：
//
//	MustWait(`() => window.__INITIAL_STATE__ !== undefined`) 在未登录时
//	永远等不到条件成立（页面停在登录墙，不注入搜索状态），
//	于是一直等到 page 的 deadline 后 panic，对外表现成
//	"context deadline exceeded" —— 一个不指向任何真实原因的错误。
//
// 实测（2026-10-21）：账号 cookie 失效 + 平台返回 300011 时，
//
//	该 MustWait 卡满 60 秒后 panic，栈顶为 rod/must.go:502。
//
// 本函数每 500ms 检查一次登录墙，命中就立刻返回具体原因；
// 只有在既没就绪、也没有登录问题时才继续等。
func WaitSearchReady(page *rod.Page, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready, err := page.Timeout(diagEvalTimeout).Eval(
			`() => (window.__INITIAL_STATE__ !== undefined) ? 1 : 0`); err == nil {
			// rod 的 Value.Int() 只返回 int（非数值时为 0），没有 ok 返回值
			if ready.Value.Int() == 1 {
				return nil
			}
		}
		if p := LoginProblem(page); p != nil {
			return fmt.Errorf("%w（%s）", p, Snapshot(page).Describe())
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("等待搜索页就绪超时（%s），且未发现登录问题——可能是平台改版或网络问题（%s）",
		timeout, Snapshot(page).Describe())
}
