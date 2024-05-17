---
title: 参与开发
sidebar_position: 2
---

如果是通过 `access token` 来测试，可以在个人仓库上，配置好hook后，用以下命令执行:

```bash
go run . -access-token=<your-access-token> -webhook-secret=<webhook-secret> -config ./config/config.yaml -log-level 0
```

如果是通过 `Github APP` 方式，执行命令为:

```bash
go run .  -webhook-secret=<webhook-secret> -config ./config/config.yaml -log-level 0 -app-id=<github_app_id>  -app-private-key=<github_app_private_key>
```