---
title: 架构与流程
sidebar_position: 3
---

`Reviewbot` 目前主要作为GitHub Webhook服务运行，会接受GitHub Events，然后执行各种检查，若检查出问题，会精确响应到对应代码上。

![architecture](./img/arch.png)

## 基本流程如下:

* 事件进来，判断是否是Pull Request
* 获取代码：
    * 获取PR影响的代码
    * clone主仓
        * 主仓会作为缓存
    * checkout PR，并放置在临时目录
* 进入Linter执行逻辑
    * 筛选linter
        * 默认只要支持的linter都对所有仓库适用，除非有单独配置
            * 单独配置需要通过配置文件显式指定
            * 显式指定的配置会覆盖默认的配置
    * 执行linter
    * 通用逻辑
        * 执行相应命令，拿到输出结果
        * filter输出的结果，只获取本次PR关心的部分
            * 有的linter关心代码
            * 有的linter关心其他
    * 做反馈
        * 有的linter给Code Comment，精确到代码行
        * 有的linter给issue comment
