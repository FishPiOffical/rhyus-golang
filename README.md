<p align = "center">
<img alt="rym" src="https://tmx.fishpi.cn/image/rym.png">
<br>
Rhythm 开源子节点系统的 go 语言实现，实现聊天室 WebSocket 多节点负载均衡，欢迎加入。

### 配置文件

| 配置项                              | 值                   | 说明                                    |
|----------------------------------|---------------------|---------------------------------------|
| **host**                         | `0.0.0.0`           | 监听地址                                  |
| **port**                         | `10831`             | WS 监听端口                               |
| **pprof.enable**                 | `true`              | 是否启用 pprof                            |
| **pprof.pprofPort**              | `10832`             | pprof 监听端口                            |
| **pprof.maxFile**                | `6`                 | 采样文件数量（maxFile * 7）                   |
| **pprof.sampleTime**             | `10`                | 采样时间（分钟）                              |
| **ssl.enable**                   | `false`             | 是否启用 SSL                              |
| **ssl.certFile**                 | `cert.pem`          | 证书文件                                  |
| **ssl.keyFile**                  | `key.pem`           | 私钥文件                                  |
| **masterUrl**                    | `https://fishpi.cn` | 主服务端地址                                |
| **adminKey**                     | `123456`            | 管理员密钥                                 |
| **heartbeat**                    | `10`                | 心跳时间间隔（秒）                             |
| **masterPoolSize**               | `8`                 | 主服务端连接池大小（**最大连接数**）                  |
| **masterNodeCacheSize**          | `8`                 | 主服务端节点缓存大小（待消费节点）                     |
| **masterMessageCacheSize**       | `32`                | 主服务端消息缓存大小（待消费消息）                     |
| **clientPoolSize**               | `32`                | 客户端连接处理协程池大小                          |
| **clientMessageHandlerPoolSize** | `32`                | 客户端消息处理协程池大小（**可用于限制处理速度和带宽**）        |
| **clientNodeCacheSize**          | `32`                | 客户端节点缓存大小（待消费节点）                      |
| **clientMessageCacheSize**       | `1024`              | 客户端消息缓存大小（待消费消息）                      |
| **goMaxProcs**                   | `20`                | Go 运行时使用的最大 CPU 核心数                   |
| **logLevel**                     | `info`              | 日志等级（info, debug, warn, error, fatal） |

