---
title: 配置
sidebar_position: 4
---

`Reviewbot` 尽可能追求 **无配置**，常见的行为都会固定到代码逻辑中。但针对一些特殊需要，也可以通过配置完成.

### 调整执行命令

linters 一般都是用默认命令执行，但是我们也可以调整命令，比如

```yaml
qbox/kodo:
  staticcheck:
    workDir: "src/qiniu.com/kodo"
```

这个配置意味着，针对`qbox/kodo`仓库代码的`staticcheck`检查，要在`src/qiniu.com/kodo`目录下执行。

### 不执行指定 Linter

比如，想在 `qbox/net-gslb` 仓库不执行`golangci-lint`检查，可以这么配置：

```yaml
qbox/net-gslb:
  golangci-lint:
    enable: false
```

### 更多的组合场景

可以通过调整 org 和 repo 的配置，来组合出更多的场景，比如

- 新增加一种 Linter 检查，想在特定仓库上灰度测试。就可以通过在 org 级别的配置关闭这个 Linter，但在目标 repo 上打开, 类似:

```yaml
qbox:
  staticcheck:
    enable: false

qbox/kodo:
  staticcheck:
    enable: true
```
