# reviewbot - Comprehensive linters runner for code review scenarios

[![Build Status](https://github.com/qiniu/reviewbot/actions/workflows/go.yml/badge.svg)](https://github.com/qiniu/reviewbot/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/reviewbot)](https://goreportcard.com/report/github.com/qiniu/reviewbot)
[![GitHub release](https://img.shields.io/github/v/tag/qiniu/reviewbot.svg?label=release)](https://github.com/qiniu/reviewbot/releases)

旨在帮助组织，建立软件工程的最佳实践并有效的推广它们。

## Why Reviewbot?

保障有限数量、有限语言的仓库代码质量是不难的，我们只需要利用各种检查工具给相关的仓库一一配置即可。但如果面临的是整个组织，各种语言，各种新旧仓库(300+)，且有很多历史遗留问题，又该如何做呢？

我们想，最好有一个中心化的静态检查服务，能在极少配置的情况下，就能应用到所有仓库，且能让每一项新增工程实践，都能在组织内高效落地。

**Reviewbot** 就是在这样的场景下诞生。

她受到了行业内很多工具的启发，但又有所不同:

- 类似 `golangci-lint`, **Reviewbot** 会是个 Linters 聚合器，但她包含更多的语言和流程规范(go/java/shell/git-flow/doc-style ...)，甚至自定义规范
- 参考 [reviewdog](https://github.com/reviewdog/reviewdog), **Reviewbot** 主要也是以 Review Comments 形式来反馈问题，精确到代码行，可以作为质量门禁，持续的帮助组织提升代码质量，比较优雅
- 推荐以 GitHub APP 或者 Webhook Server 形式部署私有运行，对私有代码友好

如果你也面临着类似的问题，欢迎尝试**Reviewbot**!

## Components

- go linters
  - [golangci-lint](/internal/linters/go/golangci_lint/)
  - [gofmt](/internal/linters/go/staticcheck/)
- c/c++ linters
  - [cppcheck](/internal/linters/c/cppcheck/)
- lua linters
  - [luacheck](/internal/linters/lua/luacheck/)
- git flow 规范
  - [commit msg check](/internal/linters/git-flow/commit-check/)
- doc 规范
  - [note check](/internal/linters/doc/note-check/)

## Quickstart

Please take a look at our [getting started guide](https://reviewbot-x.netlify.app).

## Contributing

Your contributions to Reviewbot are essential for its long-term maintenance and improvement. Thanks for supporting Reviewbot!

### Reporting Issues

If you find a bug while working with the Reviewbot, please open an issue on GitHub and let us know what went wrong. We will try to fix it as quickly as we can.

## License

Reviewbot is released under the Apache 2.0 license. See the [LICENSE](/LICENSE) file for details.
