package errors

import "errors"

// 原实现只有这两种错误，且都被用作"没抓到数据"的兜底。
// 问题：未登录、账号被风控、真的没有结果——三种完全不同的情况
// 都会落到同一句话，使用者无法判断该重新登录、换号，还是换关键词。
var ErrNoFeeds = errors.New("没有捕获到 feeds 数据")
var ErrNoFeedDetail = errors.New("没有捕获到 feed 详情数据")

// 下面是 2026-10-21 新增的、可与"没有数据"区分开的真实故障原因。
var (
	// ErrNotLoggedIn 页面停在登录墙（未登录 / cookie 失效 / 被踢下线）。
	ErrNotLoggedIn = errors.New("小红书未登录或登录已失效——需要重新扫码登录")

	// ErrAccountAbnormal 平台明确返回账号异常（风控）。
	// 换 cookie 不一定有用，通常需要换号或等风控解除。
	ErrAccountAbnormal = errors.New("小红书账号被风控（Account abnormal）——换号或等待解除，重试无效")
)
