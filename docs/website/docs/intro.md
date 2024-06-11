---
slug: /
title: Why Reviewbot?
sidebar_position: 1
---

保障有限数量、有限语言的仓库代码质量是不难的，我们只需要利用各种检查工具给相关的仓库一一配置即可。但如果面临的是整个组织，各种语言，各种新旧仓库(300+)，且有很多历史遗留问题，又该如何做呢？

我们想，最好有一个中心化的静态检查服务，能在极少配置的情况下，就能应用到所有仓库，且能让每一项新增工程实践，都能在组织内高效落地。

**Reviewbot** 就是在这样的场景下诞生。

她受到了行业内很多工具的启发，但又有所不同:

- 类似 [golangci-lint](https://github.com/golangci/golangci-lint), **Reviewbot** 会是个 Linters 聚合器，但她包含更多的语言和流程规范(go/java/shell/git-flow/doc-style ...)，甚至自定义规范
- 参考 [reviewdog](https://github.com/reviewdog/reviewdog), **Reviewbot** 主要也是以 Review Comments 形式来反馈问题，精确到代码行，可以作为质量门禁，持续的帮助组织提升代码质量，比较优雅
- 推荐以 GitHub APP 或者 Webhook Server 形式部署私有运行，对私有代码友好

如果你也面临着类似的问题，欢迎尝试**Reviewbot**!
