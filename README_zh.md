# reviewbot - build your self-hosted automated code analysis and review server easily

[![Build Status](https://github.com/qiniu/reviewbot/actions/workflows/go.yml/badge.svg)](https://github.com/qiniu/reviewbot/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/reviewbot)](https://goreportcard.com/report/github.com/qiniu/reviewbot)
[![GitHub release](https://img.shields.io/github/v/tag/qiniu/reviewbot.svg?label=release)](https://github.com/qiniu/reviewbot/releases)

Reviewbot 帮助你快速搭建一个自托管的代码分析和代码审查服务，支持多种语言和多种代码规范，尤其适合有大量私有仓库的组织。

所有的问题都会在 Pull Request 阶段，以尽可能 `Review Comments` 或`Github Annotations`形式来反馈，且精确到代码行。

- Github Check Run (Annotations)

  <div style="display: flex; justify-content: flex-start; gap: 10px;">
    <img src="./docs/static/github-check-run.png" alt="Github Check Run" width="467"/>
    <img src="./docs/static/github-check-run-annotations.png" alt="Github Check Run Annotations" width="467"/>
  </div>

- Github Pull Request Review Comments
  <div style="display: flex; justify-content: flex-start;">
    <img src="./docs/static/github-pr-review-comments.png" alt="Github Pull Request Review Comments" width="467"/>
  </div>

这种方式能帮助 PR 的作者避免在冗长的 console log 中查找问题，非常有利于问题改进。

## 目录

- [为什么选择 Reviewbot](#why-reviewbot)
- [安装](#installation)
- [支持的代码检查工具](#supported-linters)
  - [Go 语言](#go-语言)
  - [C/C++](#cc)
  - [Lua](#lua)
  - [Java](#java)
  - [Git 流程规范](#git-流程规范)
  - [文档规范](#文档规范)
- [配置](#配置)
- [如何添加新的代码检查工具](#如何添加新的代码检查工具)
- [贡献](#贡献)
- [许可证](#许可证)

## Why Reviewbot

## 安装

Please take a look at our [getting started guide](https://reviewbot-x.netlify.app).

## Supported Linters

### 支持的代码检查工具

#### Go 语言

- [golangci-lint](/internal/linters/go/golangci_lint/)
- [gofmt](/internal/linters/go/gofmt/)
- [gomodcheck](/internal/linters/go/gomodcheck/)

#### C/C++

- [cppcheck](/internal/linters/c/cppcheck/)

#### Lua

- [luacheck](/internal/linters/lua/luacheck/)

#### Java

- [pmdcheck](/internal/linters/java/pmdcheck/)
- [checkstyle](/internal/linters/java/checkstyle/)

#### Git 流程规范

- [commit msg check](/internal/linters/git-flow/commit-check/)

#### 文档规范

- [note check](/internal/linters/doc/note-check/)

## 配置

## 如何添加新的代码检查工具

## 贡献

Your contributions to Reviewbot are essential for its long-term maintenance and improvement. Thanks for supporting Reviewbot!

If you find a bug while working with the Reviewbot, please open an issue on GitHub and let us know what went wrong. We will try to fix it as quickly as we can.

## License

Reviewbot is released under the Apache 2.0 license. See the [LICENSE](/LICENSE) file for details.
