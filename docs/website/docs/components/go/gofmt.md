---
title: gofmt
sidebar_position: 2
---

所有的go代码都需经过[gofmt](https://pkg.go.dev/cmd/gofmt)格式化，已是go工程领域的既定事实。

**Reviewbot** 会执行**gofmt**检查，确保这个规范在组织内被有效贯彻。

值得注意的是，如果**Reviewbot**检测出格式问题，她会以[suggest changes](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/incorporating-feedback-in-your-pull-request)形式直接comment目标代码行，相对优雅些。

:::info
由于 check run 模式下不支持GitHub Suggestion 功能，因此，gofmt 固定使用 GitHub PR Review 风格来反馈捕获到的问题。
Issue详情参见: https://github.com/qiniu/reviewbot/issues/166
:::