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
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn libpcap-dev curl gcc
WORKDIR /
# check binary
RUN cppcheck --version \
    && shellcheck --version \
    && luacheck --version \
    && git --version \
    && ssh -V \
    && yarn --version

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