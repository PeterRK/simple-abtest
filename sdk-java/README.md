# simple-abtest Java SDK

[中文](README.md) | [English](README-EN.md)

`sdk-java`是`simple-abtest`的Java本地判定SDK，用于在业务进程内基于实验快照完成判定，判定行为与分流服务保持一致。

## 功能说明

- 初始化时同步拉取`GET /app/:id`
- 可选后台定时刷新实验快照
- 在本地完成过滤条件计算和实验判定
- 返回每个实验层的配置和命中标签

项目采用标准Maven结构，JSON解析使用Gson。

## 构建与测试

```bash
cd sdk-java
mvn test
```

## 使用

```java
import com.github.peterrk.simpleabtest.Client;
import com.github.peterrk.simpleabtest.Decision;

import java.util.Map;

public class Demo {
    public static void main(String[] args) throws Exception {
        try (Client client = Client.create(
                "http://127.0.0.1:8080", 1001, "your-token", 300)) {
            Decision result = client.ab("user-123", Map.of("country", "CN", "platform", "ios"));
            System.out.println(result.config());
            System.out.println(result.tags());
        }
    }
}
```

## 接口说明

- `Client.create(address, appid, accessToken, ttlSeconds)`：创建客户端，并立即同步拉取一次实验快照
- `client.ab(key, ctx)`：基于当前本地快照返回配置和标签
- `client.stamp()`：返回最近一次成功刷新快照的Unix时间戳
- `client.refresh()`：主动刷新一次本地快照
- `client.close()`：停止后台刷新线程

`ttlSeconds`的单位是秒：

- `ttlSeconds == 0`时，不启动后台自动刷新
- `0 < ttlSeconds <= 60`时，按60秒处理
- `ttlSeconds > 60`时，按传入值定时刷新

