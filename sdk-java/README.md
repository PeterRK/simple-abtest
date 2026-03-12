# simple-abtest Java SDK

`sdk-java` 是 `simple-abtest` 的 Java 本地判定 SDK，行为对齐 `sdk-go` 和 `sdk-cpp`：

- 初始化时同步拉取 `GET /app/:id`
- 可选后台定时刷新实验快照
- 在本地完成表达式过滤和分流判定
- 返回每个 layer 的配置和命中标签

项目采用标准 Maven 结构，JSON 解析使用 Gson。

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
