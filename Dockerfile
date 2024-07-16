FROM alpine:3.20

# if you want to install other tools, please add them here.
# Do not install unnecessary tools to reduce image size.
RUN set -eux  \
    apk update && \
    apk --no-cache add ca-certificates luacheck cppcheck shellcheck git openssh yarn libpcap-dev curl build-base && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b  /usr/local/bin v1.59.1

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
    && golangci-lint --version

COPY reviewbot /reviewbot

# SSH config
RUN mkdir -p /root/.ssh && chown -R root /root/.ssh/ &&  chgrp -R root /root/.ssh/ \
    && git config --global url."git@github.com:".insteadOf https://github.com/ \
    && git config --global url."git://".insteadOf https://
COPY deploy/config /root/.ssh/config
COPY deploy/github-known-hosts /github_known_hosts

EXPOSE 8888

ENTRYPOINT [ "/reviewbot" ]