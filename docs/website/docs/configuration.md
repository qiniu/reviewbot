---
title: 配置
sidebar_position: 4
---

`Reviewbot` 尽可能追求 **无配置**，常见的行为都会固定到代码逻辑中。但针对一些特殊需要，也可以通过配置完成.

所有可以配置项，都定义在 `config/config.go` 文件中，可以参考这个文件来配置。

以下是一些常见的配置场景:

### 调整执行命令

linters 一般都是用默认命令执行，但是我们也可以调整命令，比如

```yaml
qbox/kodo:
  linters:
    staticcheck:
      workDir: "src/qiniu.com/kodo"
```

这个配置意味着，针对`qbox/kodo`仓库代码的`staticcheck`检查，要在`src/qiniu.com/kodo`目录下执行。

我们甚至可以配置更复杂的命令，比如：

```yaml
qbox/kodo:
  linters:
    golangci-lint:
      command:
        - "/bin/sh"
        - "-c"
        - "--"
      args:
        - |
          source env.sh
          cp .golangci.yml src/qiniu.com/kodo/.golangci.yml
          cd src/qiniu.com/kodo
          export GO111MODULE=auto
          go mod tidy
          golangci-lint run --timeout=10m0s --allow-parallel-runners=true --print-issued-lines=false --out-format=line-number >> $ARTIFACT/lint.log 2>&1
```

这里的 command 和 args，与 Kubernetes Pod 的 command 和 args 类似，可以参考[Kubernetes Pod](https://kubernetes.io/docs/concepts/workloads/pods/)

**$ARTIFACT** 环境变量值得注意，这个环境变量是 `Reviewbot` 内置的，用于指定输出目录，方便排除无效干扰。因为 `Reviewbot` 最终只会关心 linters 的输出，而在这个复杂场景下，shell 脚本会输出很多无关信息，所以最好需要通过这个环境变量来指定输出目录，让 `Reviewbot` 只解析这个目录下的文件。

### 关闭 Linter

比如，想在 `qbox/net-gslb` 仓库不执行`golangci-lint`检查，可以这么配置：

```yaml
qbox/net-gslb:
  linters:
    golangci-lint:
      enable: false
```

### 通过 Docker 执行 linter

比如，想在 `qbox/net-gslb` 仓库执行 `golangci-lint` 检查，但又不想在本地安装 `golangci-lint`，可以通过配置 Docker 镜像来完成：

```yaml
qbox/net-gslb:
  linters:
    golangci-lint:
      dockerAsRunner:
        image: "golangci/golangci-lint:v1.54.2"
```

通常，你的镜像需要包含 `golangci-lint` 命令，以及 `golangci-lint` 执行时需要的所有依赖(比如 `golangci-lint` 需要 `golang` 环境，那么你的镜像需要包含 `golang`)。
