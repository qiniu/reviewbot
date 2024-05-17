# reviewbot - establish software engineering best practices and efficiently promote them within the organization.

[![Build Status](https://github.com/qiniu/reviewbot/actions/workflows/go.yml/badge.svg)](https://github.com/qiniu/reviewbot/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/reviewbot)](https://goreportcard.com/report/github.com/qiniu/reviewbot)
[![GitHub release](https://img.shields.io/github/v/tag/qiniu/reviewbot.svg?label=release)](https://github.com/qiniu/reviewbot/releases)

## What is Reviewbot?

`Reviewbot` 帮助构建一个强大、综合、高效的code reviewer. 它通过集成各种语言、各种静态分析工具、各种最佳实践，并以质量门禁的方式，持续的帮助组织提升代码质量。 

`Reviewbot` 当前无缝集成GitHub, 会提供精确到代码行的检查反馈，且反馈范围仅限在变动的部分，能让开发者非常方便的定位到具体问题。

## Components

* go linters
  * [golangci-lint](/internal/linters/go/golangci_lint/)
  * [staticcheck](/internal/linters/go/staticcheck/)
  * [gofmt](/internal/linters/go/staticcheck/)
* c/c++ linters
  * [cppcheck](/internal/linters/c/cppcheck/)
* lua linters
  * [luacheck](/internal/linters/lua/luacheck/)
* git flow 规范
  * [commit msg check](/internal/linters/git-flow/commit-check/)
* doc 规范
  * [note check](/internal/linters/doc/note-check/)

## Quickstart

Please take a look at our [getting started guide](https://reviewbot-x.netlify.app
).

## Contributing

Your contributions to Reviewbot are essential for its long-term maintenance and improvement. Thanks for supporting Reviewbot!

### Reporting Issues

If you find a bug while working with the Reviewbot, please open an issue on GitHub and let us know what went wrong. We will try to fix it as quickly as we can.

## License

Reviewbot is released under the Apache 2.0 license. See the [LICENSE](/LICENSE) file for details.