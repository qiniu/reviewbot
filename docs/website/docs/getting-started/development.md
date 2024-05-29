---
title: 参与开发
sidebar_position: 2
---

**Reviewbot** 当前设计上主要作为 [webhook server](https://docs.github.com/en/webhooks/about-webhooks)，通过接受 GitHub 事件，针对目标仓库的 PR，执行各种 linter 检查，判断代码是否符合规范。

所以，如果想在本地开发环境调试**Reviewbot**，需要准备如下:

- GitHub 认证 - 有以下两种方式
  - [personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)方式
  - [GitHub APP](https://docs.github.com/en/apps) 方式
- 启动**Reviewbot**

  ```bash
  # access token 方式
  go run . -access-token=<your-access-token> -webhook-secret=<webhook-secret> -config ./config/config.yaml -log-level 0
  # Github APP 方式
  go run .  -webhook-secret=<webhook-secret> -config ./config/config.yaml -log-level 0 -app-id=<github_app_id>  -app-private-key=<github_app_private_key>
  ```

- 测试用的 git 仓库 - 要有 admin 权限，这样可以拿到相应的 GitHub 事件
  - 参考 [如何给仓库配置 Webhook](https://docs.github.com/en/webhooks/using-webhooks/creating-webhooks)
- 本地模拟发送 GitHub 事件，可以借助工具 [phony](https://github.com/qiniu/reviewbot/tree/master/tools/phony)
