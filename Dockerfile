FROM aslan-spock-register.qiniu.io/golang:latest as builder
ENV GOPROXY https://goproxy.cn,direct
ENV TimeZone=Asia/Shanghai
# 设置工作目录
RUN mkdir /app
WORKDIR /app

# 复制 Go 程序源代码到工作目录
COPY  . .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build .
# 编译 Go 程序
#RUN rm -rf ./app/
# go lint tool dependencies `go list` `gofmt`
FROM aslan-spock-register.qiniu.io/golang:alpine

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn libpcap-dev curl openjdk11 bash build-base && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b  /usr/local/bin v1.59.1

WORKDIR /

#install open jdk

#ENV JDK_DOWNLOAD_URL https://download.java.net/java/GA/jdk18.0.2/f6ad4b4450fd4d298113270ec84f30ee/9/GPL/openjdk-18.0.2_linux-x64_bin.tar.gz
#ENV JDK_DOWNLOAD_SHA256 cf06f41a3952038df0550e8cbc2baf0aa877c3ba00cca0dd26f73134f8baf0e6
#RUN curl -fsSL "$JDK_DOWNLOAD_URL" -o jdk.tar.gz \
#    && echo "$JDK_DOWNLOAD_SHA256  jdk.tar.gz" | sha256sum -c - \
#    && tar -C /usr/local -xzf jdk.tar.gz \
#    && rm jdk.tar.gz
#ENV PATH /usr/local/jdk-18.0.2/bin:$PATH

RUN java -version
#install pmd
ENV PMD_DOWNLOAD_URL https://github.com/pmd/pmd/releases/download/pmd_releases%2F7.1.0/pmd-dist-7.1.0-bin.zip
ENV PMD_DOWNLOAD_SHA256 0d31d257450f85d995cc87099f5866a7334f26d6599dacab285f2d761c049354
RUN curl -fsSL "$PMD_DOWNLOAD_URL" -o pmd.zip \
    && echo "$PMD_DOWNLOAD_SHA256  pmd.zip" | sha256sum -c - \
    && unzip pmd.zip -d /usr/local\
    && rm pmd.zip

ENV PATH /usr/local/pmd-bin-7.1.0/bin:$PATH
#RUN pmd --version

#install stylecheck
ENV StyleCheck_DOWNLOAD_URL https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar
ENV StyleCheck_DOWNLOAD_SHA256 51c34d738520c1389d71998a9ab0e6dabe0d7cf262149f3e01a7294496062e42
RUN curl -fsSL "$StyleCheck_DOWNLOAD_URL" -o checkstyle.jar \
    && echo "$StyleCheck_DOWNLOAD_SHA256  checkstyle.jar" | sha256sum -c -

# check binary
RUN cppcheck --version \
    && shellcheck --version \
    && luacheck --version \
    && git --version \
    && ssh -V \
    && yarn --version \
    && curl --version \
    && gcc --version \
    && golangci-lint --version \
    && go version

#COPY reviewbot /reviewbot
COPY --from=builder /app/reviewbot /reviewbot

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]
