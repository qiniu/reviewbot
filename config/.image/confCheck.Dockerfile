# go lint tool dependencies `go list` `gofmt`
FROM golang:1.23.2-alpine3.20
#FROM aslan-spock-register.qiniu.io/golang:1.23.2-alpine3.20
ENV GOPROXY https://goproxy.cn,direct
ENV TimeZone=Asia/Shanghai
# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates git openssh yarn libpcap-dev curl openjdk11 bash build-base maven python3 yamllint
ENV JAVA_HOME=/usr/lib/jvm/java-11-openjdk
ENV PATH=$PATH:$JAVA_HOME/bin

#RUN update-alternatives --list java

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b  /usr/local/bin v1.61.0

WORKDIR /



# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

# set go proxy and private repo
RUN go env -w GOPROXY=https://goproxy.cn,direct \
    && go env -w GOPRIVATE=github.com/qbox,qiniu.com
#ENTRYPOINT ["java","-cp", "/source/p3c/p3c-pmd/target/p3c-pmd-2.1.1-jar-with-dependencies.jar", "net.sourceforge.pmd.PMD", "-f","emacs", "-R","rulesets/java/ali-comment.xml,rulesets/java/ali-concurrent.xml,rulesets/java/ali-constant.xml,rulesets/java/ali-exception.xml,rulesets/java/ali-flowcontrol.xml,rulesets/java/ali-naming.xml,rulesets/java/ali-oop.xml,rulesets/java/ali-orm.xml,rulesets/java/ali-other.xml,rulesets/java/ali-set.xml", "-d", "./github/workspace"]

EXPOSE 8888
