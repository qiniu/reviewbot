FROM library/golang:1.22.3 as builder

WORKDIR /app

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -v -trimpath -o /reviewbot . \ 
    && GOPATH=/go go install -ldflags="-extldflags=-static" github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1

FROM alpine:3.20 as runner

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh curl openjdk11 bash

#install pmd
ENV PMD_DOWNLOAD_URL https://github.com/pmd/pmd/releases/download/pmd_releases%2F7.1.0/pmd-dist-7.1.0-bin.zip
ENV PMD_DOWNLOAD_SHA256 0d31d257450f85d995cc87099f5866a7334f26d6599dacab285f2d761c049354
RUN curl -fsSL "$PMD_DOWNLOAD_URL" -o pmd.zip \
    && echo "$PMD_DOWNLOAD_SHA256  pmd.zip" | sha256sum -c - \
    && unzip pmd.zip -d /usr/local\
    && rm pmd.zip

ENV PATH /usr/local/pmd-bin-7.1.0/bin:$PATH

#install stylecheck
ENV StyleCheck_DOWNLOAD_URL https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar
ENV StyleCheck_DOWNLOAD_SHA256 51c34d738520c1389d71998a9ab0e6dabe0d7cf262149f3e01a7294496062e42
RUN curl -fsSL "$StyleCheck_DOWNLOAD_URL" -o /usr/local/checkstyle-10.17.0-all.jar \
    && echo "$StyleCheck_DOWNLOAD_SHA256  /usr/local/checkstyle-10.17.0-all.jar" | sha256sum -c -

#install open jdk

#ENV JDK_DOWNLOAD_URL https://download.java.net/java/GA/jdk18.0.2/f6ad4b4450fd4d298113270ec84f30ee/9/GPL/openjdk-18.0.2_linux-x64_bin.tar.gz
#ENV JDK_DOWNLOAD_SHA256 cf06f41a3952038df0550e8cbc2baf0aa877c3ba00cca0dd26f73134f8baf0e6
#RUN curl -fsSL "$JDK_DOWNLOAD_URL" -o jdk.tar.gz \
#    && echo "$JDK_DOWNLOAD_SHA256  jdk.tar.gz" | sha256sum -c - \
#    && tar -C /usr/local -xzf jdk.tar.gz \
#    && rm jdk.tar.gz
ENV JAVA_HOME=/usr/lib/jvm/java-1.8-openjdk
ENV PATH JAVA_HOME/bin:$PATH
WORKDIR /


COPY --from=builder /reviewbot /reviewbot
COPY --from=builder /usr/local/go/bin/gofmt /go/bin/golangci-lint /usr/local/bin/

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

RUN java -version
RUN java -jar /usr/local/checkstyle-10.17.0-all.jar --version
RUN pmd  --version
EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]