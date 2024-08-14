---
title: 安装部署
sidebar_position: 1
---

Reviewbot 提供以下两种方式访问GitHub:

* Github APP 方式 (推荐)
* Access Token 方式 

ReviewBot推荐使用GitHubAPP的方式进行集成，这样能更加方便的无缝代码管理流程中。本文按照GitHubAPP的方式进行集成

## 准备
在集成部署之前，我们要先了解Reviewbot需要用到的一些参数变量。
git  ssh_key: 必须，用来 clone 需要进行静态代码检仓库代码。 获取方式。
access-token：必须，用来触发使用相关githubapi       获取方式
webhook-secret:非必须，保持跟github的设置保持一致，如果github上没有设置就不用配置
githubappid- 使用githubapp方式集成时必须。 获取方式
githubappperm - 使用githubapp方式集成时必须。获取方式
其他：
golangci-config配置，非必须，在没有配置的情况下，会使用系统默认配置。配置方式参看
config，非必须，在没有配置的情况下，会使用系统默认配置。配置方式参看
golangci-config-goplus：非必须，在没有配置的情况下，会使用系统默认配置。配置方式参看
javapmdruleconfig：非必须，在没有配置的情况下，会使用系统默认配置。配置方式参看
javastylecheckruleconfig：非必须，在没有配置的情况下，会使用系统默认配置。配置方式参看

## 安装Reviewbot服务
ReviewBot的安装是支持多种方式的，支持在物理机器上安装，虚拟机上安装，容器上安装，因为其中还会涉及到运行环境的安装，本文安装推荐的docker方式进行安装。
# 构建镜像
# 部署镜像
# 设置外网映射

## 创建GitHubApp
1.创建GitHubApp，在Settings 》 Developer settings》 创建一个GitHubApp
2.设置权限
* Repository permissions
  * Checks: Read & write
  * Commit statuses: Read & write
  * Pull requests: Read & write
3.订阅事件
订阅需要的事件
* Pull Request
* Pull Request Review
* Pull Request Review Comment
* Pull Request Review Thread
* Push
* Release
* Commit Comment
4.设置webook地址,将设置好的外网映射地址配置在githubapp的webhoo地址中

## 触发检查
1. 在GitHub中 提交PR， 就能触发RevieBot运行，看到本次合并的增量代码代码检查结果和合并建议
![comments.png](images/comments.png)![detail.png](images/detail.png)
  


