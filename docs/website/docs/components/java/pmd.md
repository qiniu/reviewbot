---
title: pmd
sidebar_position: 2
---

[PMD](https://docs.pmd-code.org/pmd-doc-7.1.0/index.html) 是一款采用 BSD 协议发布的 Java 程序代码检查工具。该工具可以做到检查 Java 代码中是否含有未使用的变量、是否含有空的抓取块、是否含有不必要的对象等。
**Reviewbot** 默认使用的是PMD提供的BestPractices规则库[BestPractices](https://github.com/pmd/pmd/tree/master/pmd-java/src/main/java/net/sourceforge/pmd/lang/java/rule/bestpractices)。

默认情况下, **Reviewbot** 使用以下命令来对Java代码进行检查:

```bash
pmd check -f emacs -R bestpractices.xml  xx1.java xx2.java
```

:::info
PMD提供了许多其他的代码检查规则。
详情参见: https://github.com/pmd/pmd/tree/master/pmd-java/src/main/java/net/sourceforge/pmd/lang/java/rule
:::