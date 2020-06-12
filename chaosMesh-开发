
# 概述
chaosmesh是一款K8S环境用的混沌工程轻量级实现，支持多种混沌场景构建。
它可通过docker镜像方式部署，但如果想修改/二次开发，它的编译不是很便捷（you may try..）。
因此这里基于官方方式，拆分成几个子步骤：
* 最重的部分是第一步
* 而和代码相关是第二步
* 第三步是为了生成可部署的镜像

以下执行目录都是在 chaos-mesh/
前置步骤是：
	docker环境
	golang开发环境
	git clone https://github.com/pingcap/chaos-mesh

# 编译生成镜像
## 1.基础镜像

**剥离出，基础镜像生成部分**
命名为独立Dockerfile文件：Dockerfile.buildbase
```
FROM golang:1.14.4-alpine3.12 AS build_base

ARG HTTPS_PROXY
ARG HTTP_PROXY

RUN apk add --no-cache gcc g++ make bash git
RUN apk add --update nodejs yarn
```


**执行镜像生成**
2C8G，约耗时2小时
```
docker build -f Dockerfile.buildbase -t chaosmesh_buildbase:0.1 .
```

**执行过程输出**
```
Sending build context to Docker daemon  2.267MB
Step 1/5 : FROM golang:1.14.4-alpine3.12 AS build_base
 ---> 3289bf11c284
Step 2/5 : ARG HTTPS_PROXY
 ---> Using cache
 ---> b71cc9152dba
Step 3/5 : ARG HTTP_PROXY
 ---> Using cache
 ---> 00340bb53cfb
Step 4/5 : RUN apk add --no-cache gcc g++ make bash git
 ---> Running in 25a64b772133
fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/main/x86_64/APKINDEX.tar.gz
fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/community/x86_64/APKINDEX.tar.gz
(1/24) Installing ncurses-terminfo-base (6.2_p20200523-r0)
(2/24) Installing ncurses-libs (6.2_p20200523-r0)
(3/24) Installing readline (8.0.4-r0)
(4/24) Installing bash (5.0.17-r0)
Executing bash-5.0.17-r0.post-install
(5/24) Installing libgcc (9.3.0-r2)
(6/24) Installing libstdc++ (9.3.0-r2)
(7/24) Installing binutils (2.34-r1)
(8/24) Installing gmp (6.2.0-r0)
(9/24) Installing isl (0.18-r0)
(10/24) Installing libgomp (9.3.0-r2)
(11/24) Installing libatomic (9.3.0-r2)
(12/24) Installing libgphobos (9.3.0-r2)
(13/24) Installing mpfr4 (4.0.2-r4)
(14/24) Installing mpc1 (1.1.0-r1)
(15/24) Installing gcc (9.3.0-r2)


(16/24) Installing musl-dev (1.1.24-r8)
(17/24) Installing libc-dev (0.7.2-r3)
(18/24) Installing g++ (9.3.0-r2)
(19/24) Installing nghttp2-libs (1.41.0-r0)
(20/24) Installing libcurl (7.69.1-r0)
(21/24) Installing expat (2.2.9-r1)
(22/24) Installing pcre2 (10.35-r0)
(23/24) Installing git (2.26.2-r0)
(24/24) Installing make (4.3-r0)
Executing busybox-1.31.1-r16.trigger
OK: 218 MiB in 39 packages
Removing intermediate container 25a64b772133
 ---> 7b35c82ec76a
Step 5/5 : RUN apk add --update nodejs yarn
 ---> Running in 9b8192793f49
fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/main/x86_64/APKINDEX.tar.gz
fetch http://dl-cdn.alpinelinux.org/alpine/v3.12/community/x86_64/APKINDEX.tar.gz
(1/5) Installing brotli-libs (1.0.7-r5)
(2/5) Installing c-ares (1.16.1-r0)
(3/5) Installing libuv (1.37.0-r0)
(4/5) Installing nodejs (12.17.0-r0)
(5/5) Installing yarn (1.22.4-r0)
Executing busybox-1.31.1-r16.trigger
OK: 253 MiB in 44 packages
Removing intermediate container 9b8192793f49
 ---> 6ec726d66bc3
Successfully built 6ec726d66bc3
Successfully tagged chaosmesh_buildbase:0.1
```

**产生内容**
```
# docker images 
...
chaosmesh_buildbase                                                           0.1                 6ec726d66bc3        45 seconds ago      625MB

```

## 2.镜像+代码合集


**剥离出，业务镜像生成部分**
命名为独立Dockerfile文件：Dockerfile.buildbinary
```
FROM chaosmesh_buildbase:0.1 as build_base

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn       # 新增的，处理包下载失败问题

WORKDIR /src
COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_base AS binary_builder

ARG HTTPS_PROXY
ARG HTTP_PROXY
ARG UI
ARG SWAGGER

COPY . /src
WORKDIR /src
RUN make binary
```

**执行镜像生成**
2C8G，约耗时0.5小时，后续执行有只需2分钟
```
docker build -f Dockerfile.buildbinary -t chaosmesh_my:0.1 .
```

**执行过程输出**
```
Sending build context to Docker daemon  2.268MB
Step 1/15 : FROM chaosmesh_buildbase:0.1 as build_base
 ---> 6ec726d66bc3
Step 2/15 : ENV GO111MODULE=on
 ---> Using cache
 ---> 77f7eb612c78
Step 3/15 : ENV GOPROXY=https://goproxy.cn
 ---> Running in ff92b4113844
Removing intermediate container ff92b4113844
 ---> faaa5e678ee0
Step 4/15 : WORKDIR /src
 ---> Running in e677d7457f11
Removing intermediate container e677d7457f11
 ---> a919ebec1df6
Step 5/15 : COPY go.mod .
 ---> 1651592cc8eb
Step 6/15 : COPY go.sum .
 ---> 6a2c3e26a54a
Step 7/15 : RUN go mod download
 ---> Running in f7a15063e33a
Removing intermediate container f7a15063e33a
 ---> e406d3f23575
Step 8/15 : FROM build_base AS binary_builder
 ---> e406d3f23575
Step 9/15 : ARG HTTPS_PROXY
 ---> Running in 360b9686a970
Removing intermediate container 360b9686a970
 ---> f7a35aac4e60
Step 10/15 : ARG HTTP_PROXY
 ---> Running in 0d53c5bd7bb3
Removing intermediate container 0d53c5bd7bb3
 ---> 242f81bdf7bc
Step 11/15 : ARG UI
 ---> Running in 9a8a3a6a3aee
Removing intermediate container 9a8a3a6a3aee
 ---> 5ec849e5d942
Step 12/15 : ARG SWAGGER
 ---> Running in dd1f9766dfef
Removing intermediate container dd1f9766dfef
 ---> 646f623900e5
Step 13/15 : COPY . /src
 ---> 780dcca532a0
Step 14/15 : WORKDIR /src
 ---> Running in efe4967d715d
Removing intermediate container efe4967d715d
 ---> 39b9806a50f9
Step 15/15 : RUN make binary
 ---> Running in ebf1e23916a0
GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS="" GOARCH="" go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/go/bin/controller-gen object:headerFile=./hack/boilerplate/boilerplate.generatego.txt paths="./..."
GO15VENDOREXPERIMENT="1" CGO_ENABLED=1 GOOS="" GOARCH="" go build -ldflags '-s -w -X 'github.com/pingcap/chaos-mesh/pkg/version.buildDate=2020-06-12T09:08:08Z'' -o bin/chaos-daemon ./cmd/chaos-daemon/main.go
GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS="" GOARCH="" go build -ldflags '-s -w -X 'github.com/pingcap/chaos-mesh/pkg/version.buildDate=2020-06-12T09:08:50Z'' -o bin/chaos-controller-manager ./cmd/controller-manager/*.go
GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS="" GOARCH="" go build -ldflags '-s -w -X 'github.com/pingcap/chaos-mesh/pkg/version.buildDate=2020-06-12T09:08:58Z'' -o bin/chaosfs ./cmd/chaosfs/*.go
GO15VENDOREXPERIMENT="1" CGO_ENABLED=1 GOOS="" GOARCH="" go build -ldflags '-s -w -X 'github.com/pingcap/chaos-mesh/pkg/version.buildDate=2020-06-12T09:09:02Z'' -tags "" -o bin/chaos-dashboard cmd/chaos-dashboard/*.go
Removing intermediate container ebf1e23916a0
 ---> 166906930d82
Successfully built 166906930d82
Successfully tagged chaosmesh_my:0.1
```


**产生内容**
生成的镜像大了不少
```
# docker images
...
chaosmesh_my                                                                  0.1                 166906930d82        3 minutes ago       2.67GB
<none>                                                                        <none>              f6e5468571fe        9 minutes ago       625MB
<none>                                                                        <none>              6255c2ee2630        20 minutes ago      625MB
chaosmesh_buildbase                                                           0.1                 6ec726d66bc3        29 minutes ago      625MB
```

对比官方安装的效果：
```
# docker images
...
pingcap/chaos-dashboard                                                       latest              763cddd4c303        23 hours ago        57.5MB
pingcap/chaos-mesh                                                            latest              6e2777640799        42 hours ago        40.6MB
pingcap/chaos-daemon                                                          latest              af1cb5058c1d        42 hours ago        59.7MB
```

## 3.业务镜像
从 
### chaos-daemon

**业务镜像**
命名为独立Dockerfile文件：Dockerfile.chaos-daemon
```
FROM alpine:3.10

ARG HTTPS_PROXY
ARG HTTP_PROXY

RUN apk add --no-cache tzdata iptables ipset stress-ng iproute2

COPY --from=chaosmesh_my:0.1 /src/bin/chaos-daemon /usr/local/bin/chaos-daemon
```

**执行镜像生成**
```
docker build -f Dockerfile.chaos-daemon -t chaos-daemon:my .
```

**产生内容**
```
chaos-daemon                                                                  my                  72f767fd07a3        15 seconds ago      59.7MB
```
