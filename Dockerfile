FROM library/golang:1.22.4 as builder

WORKDIR /app

# keep this cache in a separate layer to speed up builds
RUN GOPATH=/go go install -ldflags="-extldflags=-static" github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -v -trimpath -o /reviewbot .  

FROM alpine:3.20 as runner

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
<<<<<<< HEAD
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn curl openjdk11 bash

#install open jdk

ENV JAVA_HOME=/usr/lib/jvm/java-11-openjdk
ENV PATH JAVA_HOME/bin:$PATH
=======
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn libpcap-dev curl
>>>>>>> e5cfc71 (fix golangci-lint init env (#226))
WORKDIR /
# check binary
RUN cppcheck --version \
    && shellcheck --version \
    && luacheck --version \
    && git --version \
    && ssh -V \
    && yarn --version
RUN java -version

#install pmd
ENV PMD_DOWNLOAD_URL https://github.com/pmd/pmd/releases/download/pmd_releases%2F7.1.0/pmd-dist-7.1.0-bin.zip
ENV PMD_DOWNLOAD_SHA256 0d31d257450f85d995cc87099f5866a7334f26d6599dacab285f2d761c049354
RUN curl -fsSL "$PMD_DOWNLOAD_URL" -o pmd.zip \
    && echo "$PMD_DOWNLOAD_SHA256  pmd.zip" | sha256sum -c - \
    && unzip pmd.zip -d /usr/local\
    && rm pmd.zip

ENV PATH /usr/local/pmd-bin-7.1.0/bin:$PATH
RUN pmd  --version

#install stylecheck
ENV StyleCheck_DOWNLOAD_URL https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar
ENV StyleCheck_DOWNLOAD_SHA256 51c34d738520c1389d71998a9ab0e6dabe0d7cf262149f3e01a7294496062e42
RUN curl -fsSL "$StyleCheck_DOWNLOAD_URL" -o /usr/local/checkstyle-10.17.0-all.jar \
    && echo "$StyleCheck_DOWNLOAD_SHA256  /usr/local/checkstyle-10.17.0-all.jar" | sha256sum -c -

RUN java -jar /usr/local/checkstyle-10.17.0-all.jar --version


COPY --from=builder /reviewbot /reviewbot
COPY --from=builder /go/bin/golangci-lint /usr/local/bin/
# golangci-lint dependencies
COPY --from=builder /usr/local/go/ /usr/local/go/ 

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

ENV PATH="${PATH}:/usr/local/go/bin"

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]