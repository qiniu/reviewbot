FROM library/golang:1.22.3 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN  go mod download
COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /reviewbot .

RUN GOPATH=/go go install honnef.co/go/tools/cmd/staticcheck@2023.1.6

FROM ubuntu:22.04 as runner

# if you want to install other tools, please add them here
RUN apt-get update && apt-get install -y ca-certificates luarocks cppcheck shellcheck dnsutils curl git wget vim htop jq telnet iputils-ping \
    && luarocks install luacheck \
    && rm -rf /var/lib/apt/lists/* \
    && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v1.59.0

WORKDIR /

COPY --from=builder /reviewbot /reviewbot
COPY --from=builder /go/bin/staticcheck /usr/local/bin/staticcheck

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]