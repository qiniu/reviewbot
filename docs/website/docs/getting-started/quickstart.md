---
title: 快速部署
sidebar_position: 3
---

Reviewbot 提供以下两种方式访问GitHub:

* Github APP 方式 (推荐)
* Access Token 方式 

推荐使用`Github APP`的方式,因为[Access Token 方式不支持GitHub CheckRun 姿势](https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run)

:::tip
`Github CheckRun` 姿势看起来相对优雅一些, 一家之言。
:::

创建一个`GitHub APP`也非常方便，参见:

* 基于实际情况，选择是在 Org 下创建，还是在 个人账号下创建. 
  * Org: `https://github.com/organizations/<your org>>/settings/apps`
  * 个人: `https://github.com/settings/apps`

* 设置合适的 APP的权限
  * Repository permissions
    * Checks: Read & write
    * Commit statuses: Read & write
    * Pull requests: Read & write
* 订阅需要的事件
  * Pull Request
  * Pull Request Review
  * Pull Request Review Comment
  * Pull Request Review Thread
  * Push
  * Release
  * Commit Comment

当创建完APP之后，我们就可以获得 `APP ID` 和 `APP Private Key`, 这些信息在部署时需要。

当然仍然可以是用`Access Token`方式，只不过反馈会以Comment形式存在。

创建`Access Token`请参考[GitHub官方文档](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens).

## 部署

推荐通过Docker方式，部署到kubernetes集群

* 镜像构建，请参考 [Dockerfile](https://github.com/qiniu/reviewbot/blob/master/Dockerfile)
* Kubernetes 部署: [Reviewbot.yaml](https://github.com/qiniu/reviewbot/blob/master/deploy/reviewbot.yaml)

待服务部署好之后，配置上合适的域名，然后将相关域名配置到GitHub Hook区域即可。

之后即可观察，服务是否能接受到GitHub事件，并正常执行。

