---
title: golangci-lint
sidebar_position: 1
---

[golangci-lint](https://github.com/golangci/golangci-lint) 是 go 语言领域非常优秀的 linters 执行器，她内置支持了很多 go 领域 linter 工具。

**Reviewbot** 也使用 **golangci-lint** 来规范 go 代码的编写。

默认情况下, **Reviewbot** 使用以下命令来检查 go 代码:

```bash
golangci-lint run -D staticcheck --timeout=5m0s --allow-parallel-runners=true
```

:::info
考虑到充分利用 golangci-lint 的特性，我们计划开启更多的 linters。
详情参见: https://github.com/qiniu/reviewbot/issues/116
:::
