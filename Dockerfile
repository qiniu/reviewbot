FROM golang:1.21.5 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN  go mod download
COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /cr-bot .

# install lint tools
RUN GOPATH=/go go install honnef.co/go/tools/cmd/staticcheck@2023.1.6

FROM ubuntu:20.04 as runner

RUN apt-get update && apt-get install -y ca-certificates \
    && apt-get install -y dnsutils \
    && apt-get install -y curl git wget vim htop jq telnet \
    && apt-get install -y iputils-ping \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /

COPY --from=builder /cr-bot /cr-bot
COPY --from=builder /go/bin/staticcheck /usr/local/bin/staticcheck

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts
RUN git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://

EXPOSE 8888

ENTRYPOINT [ "/cr-bot" ]