---
title: 快速部署
sidebar_position: 3
---

Reviewbot 提供以下两种方式访问GitHub:

* GitHub App 方式 (推荐)
* Access Token 方式 

Reviewbot推荐使用GitHub App的方式进行集成，这样能更加方便的无缝代码管理流程中。本文按照GitHub App的方式进行集成

### 准备



在集成部署之前，我们要先了解Reviewbot需要用到的一些参数变量。
| **名称** | **是否必须** | **用途** | **获取方式** |
|:------:|:----------:|:---------:|:------:|
| ssh-secret | 必须 | 用来 拉取待检查代码  |本地生成ssh_key私钥,公钥添加到对应GitHub账号 |
| access-token | 必须 | 用来触发使用相关GitHub API |GitHub账号setting中获取 |
| githubappid | 必须 | GitHub API使用 |GitHub App中获取 |
| githubappperm | 必须 | GitHub API使用 |创建GitHub App时设置 |
| webhook-secret |  非必须 | 验证Webhook请求的有效性 |保持跟GitHub Webhook的设置保持一致，如果GitHub上没有设置就不用配置 |

其他：
| 名称 | 是否必须 | 用户| 获取方式 |
|:------:|:-------:|:------:|:------:|
| config | 非必须 | Reviewbot配置文件|在没有配置的情况下，会使用系统默认配置。配置方式参看 |
| golangci-config | 非必须 | golang语言静态检查配置|在没有配置的情况下，会使用系统默认配置。配置方式参看 |
| javapmdruleconfig | 非必须 | java pmd 检查规则|在没有配置的情况下，会使用系统默认配置。配置方式参看 |
| javastylecheckruleconfig | 非必须 | java style check 规制|在没有配置的情况下，会使用系统默认配置。配置方式参看 |


### 安装Reviewbot服务
Reviewbot的安装是支持多种方式的，支持在物理机，虚拟机，容器上安装，因为其中还会涉及到运行环境的安装，推荐使用工程中的`Dockerfile`进行容器化的安装，步骤如下。
#### 构建镜像
1. 使编译Reviewbot文件，使用 `make all` 或者 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build .`  
2. 构建镜像：`docker build -t reviewbot-customimageid  .  `
3. tag镜像：`docker tag  reviewbot-customimageid:latest  your_dockerimage_repo/reviewbot-customimageid:latest`
4. 推送镜像：`docker push  your_dockerimage_repo/reviewbot-customimageid:latest`

#### 部署镜像
Docker 部署可以使用k8s进行部署，也可以在一台安装了docker的机器上进行部署。
- k8s部署（推荐）：  
在k8s 上创建configmap，将对应的配置设置到configmap中。 
1. 创建config  
``` shell 
kubectl create configmap cm-reviewbot --from-file=config=/Users/mac/Documents/project/reviewbot/deploy/config -n reviewbot
```
2. 设置 access-token 和 webhook-secret：  
``` shell
kubectl create secret generic github --from-literal=access-token=ghp_5vV5DueLdf0HyS9KlB4usWRJvcziK2eFFMS --from-literal=webhook-secret=910399965ee2cbb8fddad085dfda6c1cc263   -n reviewbot
```
2. 创建 ssh_sercret：  
``` shell 
kubectl create secret generic ssh-secret --from-file=ssh-secret=/Users/mac/.ssh/id_rsa -n reviewbot
```
3. 创建 app-id和app-installation-id：  
``` shell
kubectl create secret generic github-app --from-literal=app-id=957941 --from-literal=app-installation-id=53342102   -n reviewbot
```
3. 通过工程中提供的`reviewbot.yaml` 在K8S上通过命令行 `kubectl apply -f reviewbot.yaml` 进行初始化的部署  。
4.  如果镜像重新编译了，可以同通过下面命令重新设置镜像：   
``` shell
kubectl set image deployment/reviewbot reviewbot=镜像上传地址/reviewbot-新镜像:latest -n reviewbot
```
5. 启动镜像，镜像文件从configmap中读取配置。      

:::tip
如果想对Reviewbot进行其他的配置设置，根据`reviewbot.yaml`的配置要求创建对应的configmap。
:::

- 本地机器部署  
使用docker 命令启动 编译好的docker 镜像，通过参数的方式传入必须的变量信息,ssh_key文件通过mount的方式挂载到docker。  
``` shell 
docker run -p 8888:8888 --mount type=bind,target=/secrets/github_key,source=/Users/mac/.ssh/id_rsa   reviewbot-customimageid  -access-token=ghp_5vV5DueL4mx0HdddyS9KsWRJvcziK2eMS -webhook-secret=9bc cf10399965ee2cbb8fddad085dfda6c1cc263 -log-level 1
```

#### 设置外网映射
如果是通过GitHub App的方式进行集成，需要将部署好的Reviewbot服务，映射到外网ip或者域名上面，使GitHub能访问到。

### 创建GitHub App
1. 创建GitHub App，在Settings 》 Developer settings》 创建一个GitHub App,记录GitHub App ID。
2. 设置权限
   * Repository permissions
     * Checks: Read & write
     * Commit statuses: Read & write
     * Pull requests: Read & write
3. 订阅事件
订阅需要的事件
   * Pull Request
   * Pull Request Review
   * Pull Request Review Comment
   * Pull Request Review Thread
   * Push
   * Release
   * Commit Comment
4. 设置Webook地址,将设置好的外网映射地址配置在GitHub App的Webhook地址中。
5. 安装GitHub App,在Settings>Developer Settings>Install App 安装创建的GitHub App。

### 触发检查
1. 在GitHub中 提交PR， 就能触发Reviebot运行，看到本次合并的增量代码代码检查结果和合并建议。
![comments.png](images/comments.png)![detail.png](images/detail.png)

  


