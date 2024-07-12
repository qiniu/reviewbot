module github.com/qiniu/reviewbot

go 1.22.3

toolchain go1.22.5

require (
	github.com/bradleyfalzon/ghinstallation/v2 v2.8.0
	github.com/google/go-github/v57 v57.0.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/prometheus/client_golang v1.19.0
	github.com/qiniu/x v1.13.2
	github.com/sirupsen/logrus v1.9.0
	sigs.k8s.io/prow v0.0.0-20230209194617-a36077c30491
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/google/go-github/v56 v56.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.54.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	google.golang.org/protobuf v1.34.0 // indirect
	k8s.io/apimachinery v0.28.3 // indirect
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2 // indirect
)

replace sigs.k8s.io/prow => github.com/Carlji/prow v1.0.0-alpha
