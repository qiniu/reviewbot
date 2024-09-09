# go lint tool dependencies `go list` `gofmt`
FROM golang:1.23.0-alpine3.20

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn libpcap-dev curl openjdk11 bash build-base && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b  /usr/local/bin v1.60.3

WORKDIR /
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

# install docker
RUN apk add --no-cache docker docker-cli

RUN java -version
#install pmd
ENV PMD_DOWNLOAD_URL https://github.com/pmd/pmd/releases/download/pmd_releases%2F7.4.0/pmd-dist-7.4.0-bin.zip
ENV PMD_DOWNLOAD_SHA256 1dcbb7784a7fba1fd3c6efbaf13dcb63f05fe069fcf026ad5e2933711ddf5026
RUN curl -fsSL "$PMD_DOWNLOAD_URL" -o pmd.zip \
    && echo "$PMD_DOWNLOAD_SHA256  pmd.zip" | sha256sum -c - \
    && unzip pmd.zip -d /usr/local\
    && rm pmd.zip

ENV PATH /usr/local/pmd-bin-7.4.0/bin:$PATH
RUN pmd --version

#install stylecheck
ENV StyleCheck_DOWNLOAD_URL https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar
ENV StyleCheck_DOWNLOAD_SHA256 51c34d738520c1389d71998a9ab0e6dabe0d7cf262149f3e01a7294496062e42
RUN curl -fsSL "$StyleCheck_DOWNLOAD_URL" -o checkstyle.jar \
    && echo "$StyleCheck_DOWNLOAD_SHA256  checkstyle.jar" | sha256sum -c -
RUN java -jar checkstyle.jar  --version
COPY reviewbot /reviewbot


# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

# set go proxy and private repo
RUN go env -w GOPROXY=https://goproxy.cn,direct \
    && go env -w GOPRIVATE=github.com/qbox

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]