package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"strings"

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
func Snapshot(page *rod.Page) PageSnapshot {
	var snap PageSnapshot
	res, err := page.Eval(snapshotJS)
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
	if strings.Contains(lower, "/login") {
		return errors.ErrNotLoggedIn
	}
	if snap.RawCode == "300011" || strings.Contains(strings.ToLower(snap.RawMsg), "account abnormal") {
		return errors.ErrAccountAbnormal
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
