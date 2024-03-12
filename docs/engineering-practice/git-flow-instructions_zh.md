# 超实用! 从使用视角的Git协作实战，告别死记硬背

## 本文档实战Demo, 参考B站视频: https://www.bilibili.com/video/BV1uv421172t/?spm_id_from=333.788&vd_source=622c53b841a1184f63ca7930d5602ef1

## 一切从Fork开始
* Git clone => 将远程仓库下载到本地
* Git remote add  => 添加远程仓库地址

## 开启一个新功能的开发
* **推荐: 每个任务都对应独立的分支开发**
* 从主分支切出你的开发分支
    * Git checkout -b  => 切出开发分支
    * Git fetch  =>  拉出远程分支
    * Git rebase => 跟主分支保持一致

## 如何创建【优雅的】commit？
* 四步走
    * Git diff => 先浏览一遍改动符不符合你预期
    * Git status => 再看改动了哪些文件(这些文件的路径)
    * Git add 文件 => 把文件添加到Git的暂存区
    * Git commit => 真正开始提交

* 什么样的commit是优雅的？
    * **颗粒度和可读性非常重要**
        * 颗粒度不能太大
        * 一行文字能说清
    * 为更好的利于别人理解，一般格式为 <type>(<scope>): <subject>
        * 常见的type 分类:
            * feat: 新功能、新特性
            * fix: 修改 bug
            * docs: 文档修改
            * chore: 其他修改
            * ci: 持续集成相关文件修改
            * test: 测试用例新增、修改
    * 参考阅读:
       * [how to write good commit message](https://cbea.ms/git-commit/)
       * [约定式提交 - 一种用于给提交信息增加人机可读含义的规范](https://www.conventionalcommits.org/zh-hans/)

## 如何创建【规范的】PR？
* 推荐三步走
    * Git fetch
    * Git rebase
    * Git push
* **为什么推荐git fetch 和 git rebase呢?**
    * 检查你的改动跟主分支是否冲突
    * 让你的改动添加到主分支上，让合并后的主分支时间线更加的干净直观
* 什么样的PR才是规范的？
    * CI 检查必须要通过,除非失败是预期的
    * PR Title 清晰，可理解
    * 善用 PR 的Conversation区域，补充进一步的信息
        * 必要时关联相关Issue
* 要遵守什么样的Code Review礼仪？
    * 保持心态: **giving and receiving**
    * 参考阅读: [Kubernetes code review 规范](https://github.com/kubernetes/community/blob/master/contributors/guide/contributing.md#code-review)

---
> PS: 以下详细操作，请看DEMO
---

## 代码冲突了，怎么解决？
* 先解决冲突
* 再git add

## 手滑了，产生了垃圾commit记录，怎么办？
* Git rebase -i => 选择要整理的commit记录
    * 记住: 已经合并到主分支的commits不要去动
* Git push -f  => 如果已经推到远程仓库了，经过rebase整理后的commit，需要使用force push 才能重新推进去

## 提交commit之后还想改动，但又不想产生新的commit记录，怎么办？
* Git commit --amend

## 能把代码提交到别人的仓库，集成之后再往主仓库提交吗？
* 可以
    * git remote add => 将对方的仓库加入Tracking列表
    * git fetch => 拉取对方方库到本地
    * git rebase => 将你的改动合并到目标分支的最前面
    * git push => 提交到你自己的参考
    * 针对对方仓库，正常创建PR

## 当前功能还没做完，又需要紧急干另一个任务，该怎么办？
* 可以直接正常提交PR，但 PR Tittle 带上[WIP] 标记
* 当然也可以保存下当前的变动，转而切出新分支干另一个任务
    * 基本步骤
        * git stash
        * git checkout -b <new_branch>
        * git fetch
        * git rebase 
    * 如何恢复保存的临时变动？
        * git stash apply 

## 要针对线上版本做紧急Hotfix，该怎么办？
* git checkout <commit>
* git switch -c 

## 更多「该怎么办？」
可以参考 [Git 飞行规则](https://github.com/k88hudson/git-flight-rules/blob/master/README_zh-CN.md)
