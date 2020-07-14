
# 编译
## 简述
核心Makefile包括：
* Makefile，主要是入口
* Makefile.core.mk 环境设置，和主要操作(.PHONY)
* common/scripts/，大量细致操作
* tools/，上述的补充

## 拉取代码

```
mkdir -p $GOPATH/src/istio.io/istio
cd  $GOPATH/src/istio.io/istio
git clone https://github.com/istio/istio
cd istio
```

## 编译
### Makefile 修改 
**Makefile**
```屏蔽，这个很重要，其主要会影响一些go编译的环境变量
# -include Makefile.overrides.mk
```

**Makefile.core.mk**
```修改goproxy
# export GOPROXY ?= https://proxy.golang.org
export GOPROXY = https://goproxy.cn
```

### 编译
**make init**

**make docker**
（过程中遇到出错 & 需要修改的地方，参看FAQ）

# 生成结果
```
[root@k8s-master ~]# docker images
REPOSITORY                                                        TAG                                        IMAGE ID            CREATED             SIZE
istio/install-cni                                                 7637c3f9f4a20a163a62166544a61bb724df66f5   f1766aad6d66        20 minutes ago      223MB
istio/operator                                                    7637c3f9f4a20a163a62166544a61bb724df66f5   14ca8786191e        20 minutes ago      244MB
istio/istioctl                                                    7637c3f9f4a20a163a62166544a61bb724df66f5   06fe55eca348        21 minutes ago      272MB
istio/mixer_codegen                                               7637c3f9f4a20a163a62166544a61bb724df66f5   555dad96d372        21 minutes ago      223MB
istio/mixer                                                       7637c3f9f4a20a163a62166544a61bb724df66f5   e214cd046774        21 minutes ago      128MB
istio/test_policybackend                                          7637c3f9f4a20a163a62166544a61bb724df66f5   2e62a8b9ed5b        21 minutes ago      193MB
istio/app_sidecar_debian_10                                       7637c3f9f4a20a163a62166544a61bb724df66f5   918bbcc6658c        21 minutes ago      443MB
istio/app_sidecar_debian_9                                        7637c3f9f4a20a163a62166544a61bb724df66f5   23c23bd23815        21 minutes ago      428MB
istio/app_sidecar_ubuntu_focal                                    7637c3f9f4a20a163a62166544a61bb724df66f5   5c14f607b3a4        21 minutes ago      403MB
istio/app_sidecar_ubuntu_bionic                                   7637c3f9f4a20a163a62166544a61bb724df66f5   acf59e6f6b39        22 minutes ago      408MB
istio/app_sidecar_ubuntu_xenial                                   7637c3f9f4a20a163a62166544a61bb724df66f5   5e724a16f02e        22 minutes ago      466MB
```

# FAQ
## 拉取不到的镜像
借用阿里云+github编译，可参看 [这里](https://www.freesion.com/article/6563156615/)

### docker login 
用于登录的用户名为阿里云账号全名，密码为开通服务时设置的密码。
```
docker login --username=xxxxxx registry.cn-zhangjiakou.aliyuncs.com
```

### 镜像一

tag的具体名称，和istio具体版本的脚本有关，下面这个带日期的就经常变更，一两周就递进一次吧（所以自己完成这个镜像拉取，还是很有必要的。当然，也可以尝试用旧版本镜像来编译）
```
docker pull registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:v0.1

docker tag  registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:v0.1 gcr.io/istio-testing/build-tools:master-2020-07-08T14-39-36
```

### 镜像二
```
docker pull registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:cc-v0.1

docker tag  registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:cc-v0.1 gcr.io/distroless/cc
```

tag后，镜像的摘要信息有问题，导致必须做以下处理：
``` 
(不止一处)把编译出错提示中，摘要信息，从编译脚本中删去....
FROM gcr.io/distroless/cc@sha256:f81e5db8287d66b012d874a6f7fea8da5b96d9cc509aa5a9b5d095a604d4bca1 as distroless
    改为
FROM gcr.io/distroless/cc as distroless
```


### 镜像三
```
docker pull registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:static-debian10-v0.1

docker tag  registry.cn-zhangjiakou.aliyuncs.com/com_ka_img/istio:static-debian10-v0.1 gcr.io/distroless/static-debian10
```

（同上）tag后，镜像的摘要信息有问题，导致必须做以下处理：
``` 
(不止一处)把编译出错提示中，摘要信息，从编译脚本中删去....
FROM FROM gcr.io/distroless/static-debian10@sha256:4433370ec2b3b97b338674b4de5ffaef8ce5a38d1c9c0cb82403304b8718cde9
   改为
FROM gcr.io/distroless/static-debian10
```


## 安装fpm
### 
https://www.iyunv.com/thread-982376-1-1.html
但centos默认自带的ruby版本过低

### ruby升级到>2.3版本
https://www.cnblogs.com/lylongs/p/11302272.html
