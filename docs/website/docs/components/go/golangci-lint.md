---
title: golangci-lint
sidebar_position: 1
---

[golangci-lint](https://github.com/golangci/golangci-lint) 是 go 语言领域非常优秀的 linters 执行器，她内置支持了很多 go 领域 linter 工具。

**Reviewbot** 也使用 **golangci-lint** 来规范 go 代码的编写。

### 执行逻辑

从简化配置以及适配结果的角度，执行器有两类核心逻辑:

- **缺省模式** 如果没有设置 Command, 或者 Command 唯一且为`golangci-lint`, 那么 Args 参数在子命令是`run`的情况下，会做如下检验(其他子命令，不会做处理)

  - 如果没有设置`--timeout`, 那么默认设置为 `--timeout=5m0s`
  - 如果没有设置`--allow-parallel-runners`, 那么默认设置为 `--allow-parallel-runners=true`
  - 如果没有设置`--out-format`, 那么默认设置为 `--out-format=line-number`
  - 如果没有设置`--print-issued-lines`, 那么默认设置为 `--print-issued-lines=false`

- **自定义模式** 如果设置了 Command, 且 Command 不为`golangci-lint`, 此模式下执行器将不会做任何的验证和补充，将按照配置的内容严格执行

  - 此模式一般应用于比较复杂的项目，此类项目一般需要在执行命令前做一些前置工作
  - 对于结果解析，执行器会通过模式 **`^(._?):(\d+):(\d+)?:? (._)$`** 匹配所有的输出行，符合的情况会做相应上报. 但这种模式下有可能会有很多不预期的输出内容，有可能会干扰日常的监测运营。所以，更推荐的把 golangci-lint 的输出内容重定向到 **$ARTIFACT** 目录下，执行器会优先解析这个目录下的内容。

  - 参考例子:

    ```yaml
    qbox/kodo-ops:
      golangci-lint:
        enable: true
        comamnd:
          - /bin/bash
          - -c
          - --
        args:
          - cd web && yarn build
          - golangci-lint run --enable-all --timeout=5m0s --allow-parallel-runners=true >> $ARTIFACTS/lint.log 2>&1
    ```

:::info
Command 和 Args 的使用姿势跟 Kubernetes Pod 中的 Command 和 Args 的 Yaml 用法一致
:::

### golangci.yml 配置文件

可以选择继承全局的配置文件，也可以针对特定仓库选择自己的配置文件。

从使用维度考虑:

- 如果仓库中包含 `.golangci.yml`配置，将优先使用该配置
- 如果仓库中不包含相关配置，那么可以选择从全局或者组织下继承配置。当然，不管全局还是组织，都要保证目标配置文件要在相关执行执行的目录下存在。

  - 全局设置的例子

    ```yaml
    globalDefaultConfig: # global default settings, will be overridden by qbox org and repo specific settings if they exist
      golangcilintConfig: "config/linters-config/.golangci.yml" # golangci-lint config file to use
    ```

  - 从组织维度
    ```yaml
    qbox:
      golangci-lint:
        enable: true
        configPath: "config/linters-config/.golangci.yml" // TODO(CarlJi): 路径检查
    ```

### 自动选择执行目录

默认情况下，执行器会在仓库的根目录下执行，但是对于很多 monorepo 来讲，其相关的 go 代码却在特定的目录下。这时候，可能会遇到如下错误:

```bash
Unexpected: level=error msg="[linters_context] typechecking error: pattern ./...: directory prefix . does not contain main module or its selected dependencies"
```

执行器会自动分析该类型错误，尝试判断合适的目标，然后重新执行。
