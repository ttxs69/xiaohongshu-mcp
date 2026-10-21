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

	// ErrNoteBlocked 笔记本身被拦截（不是账号问题）。
	//
	// 实测（2026-10-21）：URL 为
	//   /404?source=/404/sec_alaRUWwP?redirectPath=...&error_code=300031
	//   &error_msg=当前笔记暂时无法浏览
	// 标题是「小红书 - 你访问的页面不见了」。
	// 这类拦截是**针对该条笔记**的（违规/私密/风控命中），
	// 与账号是否被封、是否需要重新登录都无关——
	// 换号或重登对它没有帮助，只能换一条笔记。
	ErrNoteBlocked = errors.New("该笔记被平台拦截（error_code=300031）——换一条笔记即可，与账号无关")

	// ErrCaptchaRequired 平台要求安全验证（验证码 / 滑块）。
	// 这不是"没登录"，而是登录后被风控要求人工验证；
	// 自动化脚本重试多少次都不会通过，必须有人在浏览器里点一次。
	//
	// 实测（2026-10-21）：搜索被重定向到
	//   https://www.xiaohongshu.com/website-login/captcha?verifyType=124&verifyBiz=461
	// 标题「安全验证」。原实现在 MustWait 里死等 60s，
	// 最后只报 "context deadline exceeded"，完全看不出是这个原因。
	ErrCaptchaRequired = errors.New("小红书要求安全验证（验证码/滑块）——需要人工在浏览器中完成一次验证后再重试")
)
