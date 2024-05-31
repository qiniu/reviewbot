---
title: stylecheck
sidebar_position: 1
---

[CheckStyle](https://github.com/checkstyle/checkstyle) 是 SourceForge 下的一个项目，提供了一个帮助 JAVA 开发人员遵守某些编码规范的工具。它能够自动化代码规范检查过程，从而使得开发人员从这项重要，但是枯燥的任务中解脱出来。CheckStyle提供了大部分功能都是对于代码规范的检查。
**Reviewbot** 默认使用的是sun提供的规则库[sun_style](https://checkstyle.org/sun_style.html)。

默认情况下, **Reviewbot** 使用以下命令来对Java代码进行stylechck检查:

```bash
java -jar checkstyle-10.17.0-all.jar run -c sun_checks.xml xx1.java xx2.java
```

:::info
Checkstyle提供了需求的代码风格检查规则。
详情参见: https://checkstyle.org/checks.html
:::