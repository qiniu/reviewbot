## Code Review Comments Learning

## Details

* https://github.com/goplus/community/pull/153#discussion_r1495351302
    * URI的设计非常讲究，需考虑最佳实践
    * 一些有价值的实践参考
        * Github API: https://docs.github.com/en/rest
        * Kubernetes API: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#api-overview
        * Aws s3 API: https://docs.aws.amazon.com/AmazonS3/latest/API/API_Operations_Amazon_Simple_Storage_Service.html
* https://github.com/goplus/community/pull/83#discussion_r1481246832
    * 锁的使用，要尤其注意粒度，越小越好，职责要单一
* https://github.com/goplus/community/pull/148#discussion_r1494611359
    * 推荐使用统一的log库
    * 作为一个集体，工程规范应尽可能统一
    * https://github.com/qiniu/x 有很多好用的库

* https://github.com/goplus/community/pull/148#discussion_r1494609866
  * 代码风格，尽量遵循Go的风格，尤其是命名，减少map等不固定长度的数据结构作为参数或者返回值
* https://github.com/goplus/community/pull/148#discussion_r1494612992
  * 单元测试，尽量覆盖所有的分支，使用assert抛出问题
* https://github.com/goplus/community/pull/168/files#r1497894587
  * 前端部分代码需要保证本地测试通过
* https://github.com/goplus/community/pull/127#issuecomment-1932663028
  * Commit记录，尽量保持清晰，不要包含无关的信息，多用rebase少用merge
* https://github.com/goplus/community/pull/108#discussion_r1478493291
  * 保证comment的质量，以及合并的代码comment不用中文
* https://github.com/goplus/community/pull/100#discussion_r1479140319
  * 避免无意义的提交，暂存的代码不要提交，可以放本地gitignore
* https://github.com/goplus/community/pull/168#discussion_r1498561221
  * 看起来是个测试代码，正式提交时需要注释，或者是需要的代码，但是src部分是写死的
* https://github.com/goplus/community/pull/168#discussion_r1498561761
  * push之前需同步
* https://github.com/goplus/community/pull/159#discussion_r1498565877
  * 减少打包文件的提交，将用到的文件上传到需要的位置
* https://github.com/goplus/community/pull/164
  * 过多的魔法值："auto",抽出当常量使用

## Reference

* [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
