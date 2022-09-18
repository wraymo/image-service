module github.com/dragonflyoss/image-service/contrib/nydusify

go 1.14

require (
	github.com/aliyun/aliyun-oss-go-sdk v2.1.5+incompatible
	github.com/baiyubin/aliyun-sts-go-sdk v0.0.0-20180326062324-cfa1a18b161f // indirect
	github.com/containerd/containerd v1.6.6
	github.com/google/go-containerregistry v0.11.0
	github.com/docker/cli v20.10.17+incompatible
	github.com/docker/distribution v2.8.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/google/uuid v1.3.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/pkg/errors v0.9.1
	github.com/pkg/xattr v0.4.3
	github.com/prometheus/client_golang v1.12.2
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.0
	github.com/tidwall/gjson v1.9.3
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f
	lukechampine.com/blake3 v1.1.5
)

replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.2
