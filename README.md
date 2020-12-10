# MicroAgent
___
 
自用golang的Agent 框架，插件式，消息通过protobuf 进行序列化/反序列化
 
### 代码架构
 
 ![图片](https://github.com/zyxpaomian/mypic/blob/main/agent.png)
 
### msg消息体生成
> 消息体使用protobuf 封装, 原文件在protobuf下，msg需要手动生成
```shell
protoc --go_out=./msg/ protobuf/agent.proto
mv msg/protobuf/agent.pb.go msg/
rm -rf msg/protobuf
```

### 安装
``` shell
go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,direct
go env -w GO111MODULE=on

# 运行
make run 

# 编译
make compile
```
 
### 待完成
* 支持Kafka 和 Server 端交互，中间件解耦
* 配置文件热加载
* 待补充