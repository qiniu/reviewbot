FROM library/golang:1.22.3 as builder

WORKDIR /app

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -v -trimpath -o /reviewbot . \ 
    && GOPATH=/go go install honnef.co/go/tools/cmd/staticcheck@2023.1.6 \
    && GOPATH=/go go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1

FROM alpine:3.20 as runner

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git
WORKDIR /

COPY --from=builder /reviewbot /reviewbot
COPY --from=builder /go/bin/staticcheck /go/bin/golangci-lint /usr/local/bin/

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]