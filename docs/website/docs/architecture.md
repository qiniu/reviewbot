---
title: 架构与流程
sidebar_position: 3
---

`Reviewbot` 目前主要作为 GitHub Webhook 服务运行，会接受 GitHub Events，然后执行各种检查，若检查出问题，会精确响应到对应代码上。

![architecture](./img/arch.png)

## 基本流程如下:

- 事件进来，判断是否是 Pull Request
- 获取代码：
  - 获取 PR 影响的代码
  - clone 主仓
    - 主仓会作为缓存
  - checkout PR，并放置在临时目录
  - pull 子模块
    - 仓库若使用submodule管理则自动拉取代码
- 进入 Linter 执行逻辑
  - 筛选 linter
    - 默认只要支持的 linter 都对所有仓库适用，除非有单独配置
      - 单独配置需要通过配置文件显式指定
      - 显式指定的配置会覆盖默认的配置
  - 执行 linter
  - 通用逻辑
    - 执行相应命令，拿到输出结果
    - filter 输出的结果，只获取本次 PR 关心的部分
      - 有的 linter 关心代码
      - 有的 linter 关心其他
  - 做反馈
    - 有的 linter 给 Code Comment，精确到代码行
    - 有的 linter 给 issue comment

## 如何添加新的 Linter？

- 请从 [issues](https://github.com/qiniu/reviewbot/issues) 列表中选择你要处理的 Issue.
  - 当然，如果没有，你可以先提个 Issue，描述清楚你要添加的 Linter
- 编码
  - 基于 linter 关注的语言或领域，[选好代码位置](https://github.com/qiniu/reviewbot/tree/master/internal/linters)
  - 绝大部分的 linter 实现逻辑分三大块:
    - 执行 linter，一般是调用相关的可执行程序
    - 处理 linter 的输出，我们只会关注跟本次 PR 相关的输出
    - 反馈 跟本次 PR 相关的输出，精确到代码行
- 部署，如果你的 linter 是外部可执行程序，那么就需要在 [Dockerfile](https://github.com/qiniu/reviewbot/blob/master/Dockerfile) 中添加如何安装这个 linter
- 文档，为方便后续的使用和运维，我们应当 [在这里](https://github.com/qiniu/reviewbot/tree/master/docs/website/docs/components) 添加合适的文档
