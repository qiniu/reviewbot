TODO:
1. 尝试跑通基于PRcomment的逻辑
   - 本地怎么测试？
   - 弄一个PR，然后找到事件，发事件测试
   - 要测试的点:
     - 正常创建comment
     - 再次运行，不会重复创建
     - 再次运行，过时的comment会被自动删除 or 更新
   - 针对单仓库测试
     - 配置webhook
   - 针对org测试
     - 创建并安装github app
   - 支持 Annotation 模式，commit 相关的文件都会被comment
2. 推进其他的lint工具   
