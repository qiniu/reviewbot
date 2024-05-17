# reviewbot - establish software engineering best practices and efficiently promote them within the organization.

[![Build Status](https://github.com/qiniu/reviewbot/actions/workflows/go.yml/badge.svg)](https://github.com/qiniu/reviewbot/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/reviewbot)](https://goreportcard.com/report/github.com/qiniu/reviewbot)
[![GitHub release](https://img.shields.io/github/v/tag/qiniu/reviewbot.svg?label=release)](https://github.com/qiniu/reviewbot/releases)

## What is Reviewbot?

`Reviewbot` helps build a powerful, comprehensive, and efficient code reviewer. It integrates various languages, static analysis tools, and best practices to continuously assist organizations in improving code quality with the implementation of a quality gate.

`Reviewbot` seamlessly integrates with GitHub, providing precise feedback on code lines and limiting the feedback scope to the changed portions only. This allows developers to easily pinpoint specific issues.
## Components

* go linters
  * [golangci-lint](/internal/linters/go/golangci_lint/)
  * [staticcheck](/internal/linters/go/staticcheck/)
  * [gofmt](/internal/linters/go/staticcheck/)
* c/c++ linters
  * [cppcheck](/internal/linters/c/cppcheck/)
* lua linters
  * [luacheck](/internal/linters/lua/luacheck/)
* git collaboration guidelines
  * [commit msg check](/internal/linters/git-flow/commit-check/)
* doc guidelines
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