package xiaohongshu

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// feedReadyTimeout 等待详情页关键容器的上限，超时不算失败。
const feedReadyTimeout = 8 * time.Second

// feedReadySelectors 详情页已出结果的判据：正常笔记容器，或 checkPageAccessible 认的错误容器。
const feedReadySelectors = ".interact-container, .note-scroller, " +
	".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper"

// waitFeedPageReady 等待 feed 详情页可用：load 事件 + 关键容器出现，不要求整页 DOM 静止。
// waitFeedPageReady 等详情页加载完成。
//
// 原实现用 page.MustWaitLoad()，失败即 panic，而 panic 信息只有
// "context deadline exceeded"——不指向真实原因。改为返回 error，
// 并在加载完成后先判登录墙（未登录时页面能"加载成功"但内容为空，
// 最终会被误报成「没有捕获到 feed 详情数据」）。
func waitFeedPageReady(page *rod.Page) error {
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("详情页加载失败: %w（%s）", err, Snapshot(page).Describe())
	}

	if p := LoginProblem(page); p != nil {
		return fmt.Errorf("%w（%s）", p, Snapshot(page).Describe())
	}

	if _, err := page.Timeout(feedReadyTimeout).Element(feedReadySelectors); err != nil {
		// 没等到也继续，后续 Element 各自带轮询
		logrus.Warnf("详情页关键容器未在 %s 内出现，继续尝试: %v", feedReadyTimeout, err)
	}
	return nil
}
