FROM aslan-spock-register.qiniu.io/library/golang:1.21.5 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN  go mod download
COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /reviewbot .

# install staticcheck lint tools
RUN GOPATH=/go go install honnef.co/go/tools/cmd/staticcheck@2023.1.6

RUN apt-get update && apt-get install -y cppcheck

FROM aslan-spock-register.qiniu.io/library/ubuntu:22.04 as runner

RUN apt-get update && apt-get install -y ca-certificates \
    && apt-get install -y dnsutils \
    && apt-get install -y curl git wget vim htop jq telnet \
    && apt-get install -y iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# install luacheck lint tools
RUN apt-get update && apt-get install -y luarocks \
    && luarocks install luacheck \
    && rm -rf /var/lib/apt/lists/*


# 设置golang环境
ENV GOLANG_DOWNLOAD_URL https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e
RUN curl -fsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
    && echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf golang.tar.gz \
    && rm golang.tar.gz

ENV PATH /usr/local/go/bin:$PATH

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/go/bin v1.56.2

WORKDIR /

COPY --from=builder /reviewbot /reviewbot
COPY --from=builder /go/bin/staticcheck /usr/local/bin/staticcheck

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts
RUN git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]