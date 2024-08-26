## Proposal: 日志持久化和展示

### Background

> see [Issue](https://github.com/qiniu/reviewbot/issues/204)


随着服务的有序进行，日志服务的持久化和查询展示必不可少。而当前服务日志未支持相应的功能。因此本提案目标为建设reviewbot的日志服务。
一、日志的持久化存储方式。 
二、日志可查询展示


### 设计方案

- config GlobalConfig 中新增字段FullLogConfig，类型为LogConfig结构。用来判断是否开启全日志持久化存储和远端存储厂商配置。
  - config.go 变量新增：
    ```go
    type GlobalConfig struct {
        ...
        ...
        FullLogConfig LogConfig 
    }

    type LogConfig  struct {
        Enable bool // 默认为开启状态
        StorageName []string // git ,s3 ,qiniu 等等

        //remote 厂商配置可多选
        Gitcfg GitConfig 
        S3cfg  S3config
        QiNiucfg  QiNiuconfig
        ...
    }
    ```


- linters.go 中 Agent 新增 LogStorages 参数用来执行日志存储和读取操作
  - linters.go 变量新增
     ```go
      type Agent struct {
          ...
          ...
          LogStorages []Storage // 支持多个storage 
      }

     ```

- storage 目录新增结构
  ```go
    type Storage interface {
        Writer(ctx context.Context, path string, content []bytes) error

        Reader(ctx context.Context,path string) error
    }

    type XXXStorage struct{
       ...
    }
    
    func NewXXXStorage()Storage{
      ...
      return XXXStorage
    }
  ```
###### 主执行逻辑
- 在主 srever handle 中判断是否开启全局日志，若开启，读取 LogConfig.RemoteName 初始化所有的 LogStorageSvc 并赋值到 linter.agent 中
- 存储：在执行器 ExecRun a.Runner.Run() 生成 reader 结果后，将所有的 LogStorage 执行 Writer()（默认不配置本地会存一份）,  此处要设计相应的path
  - path 可以根据reviewbot-logs/lintername/uniqid组成
- 读取展示
  - 1、边存边读：将 uniqid 与 io.ReadCloser 进行一一映射，当请求类似 log?id=uniqid 时，展示当前内容
  - 2、归档文件读取，在执行LogStorage 中，优先选择本地存储，若找不到，则随机选择remote 进行 LogStorage.Reader()方法进行读取，若成功，则展示，若失败，继续执行下一个LogStorage，直至成功.
  - 3、check run 地址回填
    - 当 linter 还在运行时，回填边存边读地址
    - 当 linter 运行结束时，回填归档地址
