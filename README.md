# reviewbot - Comprehensive linters runner for code review scenarios

[![Build Status](https://github.com/qiniu/reviewbot/actions/workflows/go.yml/badge.svg)](https://github.com/qiniu/reviewbot/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/reviewbot)](https://goreportcard.com/report/github.com/qiniu/reviewbot)
[![GitHub release](https://img.shields.io/github/v/tag/qiniu/reviewbot.svg?label=release)](https://github.com/qiniu/reviewbot/releases)

Its ultimate goal is to establish software engineering best practices and efficiently promote them within the organization.

## Why Reviewbot?

Ensuring code quality for a limited number of repositories and languages is not difficult. We just need to use various inspection tools and configure them for the relevant repositories. But what if we are dealing with an entire organization, with different languages, a variety of new and old repositories (300+), and many historical legacy issues? How can we handle that?

We thought it would be best to have a centralized static analysis service that can be applied to all repositories with minimal configuration and enable efficient implementation of every new engineering practice within the organization.

**Reviewbot** was created for such a scenario.

It has been inspired by many tools in the industry, but with some differences:

- Similar to [golangci-lint](https://github.com/golangci/golangci-lint), **Reviewbot** is an aggregator of linters, but it includes more languages and process specifications (go/java/shell/git-flow/doc-style, etc.), and even custom specifications.
- Inspired from [reviewdog](https://github.com/reviewdog/reviewdog), **Reviewbot** primarily provides feedback in the form of review comments, pinpointing the issues down to specific lines of code. It can serve as a quality gate and continuously help the organization improve code quality, offering an elegant solution.
- Recommend deploying **Reviewbot** as a GitHub App or a Webhook Server for private execution, ensuring compatibility with private code repositories.

If you are facing similar challenges, we welcome you to try **Reviewbot**!

## Components

- go linters
  - [golangci-lint](/internal/linters/go/golangci_lint/)
  - [gofmt](/internal/linters/go/staticcheck/)
- c/c++ linters
  - [cppcheck](/internal/linters/c/cppcheck/)
- lua linters
  - [luacheck](/internal/linters/lua/luacheck/)
- git collaboration guidelines
  - [commit msg check](/internal/linters/git-flow/commit-check/)
- doc guidelines
  - [note check](/internal/linters/doc/note-check/)

## Quickstart

Please take a look at our [getting started guide](https://reviewbot-x.netlify.app).

## Contributing

Your contributions to Reviewbot are essential for its long-term maintenance and improvement. Thanks for supporting Reviewbot!

### Reporting Issues

If you find a bug while working with the Reviewbot, please open an issue on GitHub and let us know what went wrong. We will try to fix it as quickly as we can.

## License

Reviewbot is released under the Apache 2.0 license. See the [LICENSE](/LICENSE) file for details.
