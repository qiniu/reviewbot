# go lint tool dependencies `go list` `gofmt`
FROM golang:1.22.3-alpine3.20

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates git openssh yarn libpcap-dev curl openjdk11 bash build-base  

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b  /usr/local/bin v1.59.1 

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

EXPOSE 8888
