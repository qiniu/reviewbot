---
title: 架构
sidebar_position: 3
---

`Reviewbot` 目前主要作为GitHub Webhook服务运行，会接受GitHub Events，然后执行各种检查，若检查出问题，就会创建精确响应到对应代码上。

![architecture](./img/arch.png)
