FROM aslan-spock-register.qiniu.io/library/golang:1.21.5 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN  go mod download
COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /reviewbot .

# install staticcheck lint tools
RUN GOPATH=/go go install honnef.co/go/tools/cmd/staticcheck@2023.1.6

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

# install cppcheck lint tools
RUN apt-get update && apt-get install -y cppcheck \
    && rm -rf /var/lib/apt/lists/*

#install jdk
#RUN apt-get update && apt-get install -y openjdk-8-jdk \
#    && rm -rf /var/lib/apt/lists/*

# install golang
ENV GOLANG_DOWNLOAD_URL https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e
RUN curl -fsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
    && echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf golang.tar.gz \
    && rm golang.tar.gz

ENV PATH /usr/local/go/bin:$PATH

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
RUN curl -fsSL "$StyleCheck_DOWNLOAD_URL" -o checkstyle.jar \
    && echo "$StyleCheck_DOWNLOAD_SHA256  checkstyle.jar" | sha256sum -c -

#install open jdk

ENV JDK_DOWNLOAD_URL https://download.java.net/java/GA/jdk18.0.2/f6ad4b4450fd4d298113270ec84f30ee/9/GPL/openjdk-18.0.2_linux-x64_bin.tar.gz
ENV JDK_DOWNLOAD_SHA256 cf06f41a3952038df0550e8cbc2baf0aa877c3ba00cca0dd26f73134f8baf0e6
RUN curl -fsSL "$JDK_DOWNLOAD_URL" -o jdk.tar.gz \
    && echo "$JDK_DOWNLOAD_SHA256  jdk.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf jdk.tar.gz \
    && rm jdk.tar.gz

ENV PATH /usr/local/jdk-18.0.2/bin:$PATH


RUN mkdir /usr/local/rulesets
#down pdm rules
ENV PMDRule_DOWNLOAD_URL https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml
ENV PMDRule_DOWNLOAD_SHA256 51a2bc383c2708afb8e41925947c3a6a5218e5bbdaf495142500a12aa29df314
RUN curl -fsSL "$PMDRule_DOWNLOAD_URL" -o /usr/local/rulesets/bestpractices.xml \
    && echo "$PMDRule_DOWNLOAD_SHA256  /usr/local/rulesets/bestpractices.xml" | sha256sum -c
    #down checkstyle rules
ENV StyleCheckRule_DOWNLOAD_URL https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml
ENV StyleCheckRule_DOWNLOAD_SHA256 c763b24a66cf51a392b13ba289d7c619d5bf4f7ae2ca5c2a324cfb57a6dd47a8
RUN curl -fsSL "$StyleCheckRule_DOWNLOAD_URL" -o /usr/local/rulesets/sun_checks.xml \
    && echo "$StyleCheckRule_DOWNLOAD_SHA256  /usr/local/rulesets/sun_checks.xml" | sha256sum -c


RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/go/bin v1.56.2

# install shellcheck lint tools
RUN apt-get update && apt-get install -y shellcheck \
    && rm -rf /var/lib/apt/lists/*  

# install python flake8  lint tools
RUN apt update && apt install -y  flake8 \
    && rm -rf /var/lib/apt/lists/*

#install eslint

RUN apt update && apt install -y  nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN apt update && apt install -y  npm \
    && rm -rf /var/lib/apt/lists/*

RUN  npm install -y  eslint \
    && rm -rf /var/lib/apt/lists/*

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