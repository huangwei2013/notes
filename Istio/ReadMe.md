# 背景

Istio 镜像模式运行，一个需要资源，一个调试也不方便。于是想通过二进制方式本地运行(当然是在linux上)

于是先折腾了一个月 Istio 基本编译环境，混眼熟了代码...(效率的确不高)

然后基于 Istio1.7.0 的 ./pilot/pkg/xds/ 下的各个 xx_test.go，改造成 Istio1.7.0/my/ 下的 TestXX.go 


# HowTo
## 环境说明
Centos7.6  2C8G

## 运行


1.Istio 本地编译 
参看 https://my.oschina.net/kakablue 

2.运行场景测试

```
go run 你看的上的测试场景.go
```
