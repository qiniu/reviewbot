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






## Reference

* [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
