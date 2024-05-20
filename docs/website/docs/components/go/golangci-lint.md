---
title: golangci-lint
sidebar_position: 1
---

[golangci-lint](https://github.com/golangci/golangci-lint) 是go语言领域非常优秀的linters执行器，她内置支持了很多go领域linter工具。

**Reviewbot** 也使用 **golangci-lint** 来规范go代码的编写。

默认情况下, **Reviewbot** 使用以下命令来检查go代码:

```bash
golangci-lint run -D staticcheck
```

:::info
考虑到充分利用golangci-lint的特性，我们计划开启更多的linters。
详情参见: https://github.com/qiniu/reviewbot/issues/116
:::