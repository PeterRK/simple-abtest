# simple-abtest C++ SDK

`sdk-cpp`是`simple-abtest`的C++ 本地判定SDK，行为对齐`sdk-go`：

- 初始化时同步拉取`GET /app/:id`
- 可选后台定时刷新实验快照
- 在本地完成表达式过滤和分流判定
- 返回每个layer的配置和命中标签

## 依赖

- C++17
- `libcurl`
- `zlib`
- `protobuf` / `protoc`
- `gtest`（仅测试）

Ubuntu/Debian示例：

```bash
sudo apt-get install -y cmake g++ libcurl4-openssl-dev zlib1g-dev protobuf-compiler libprotobuf-dev libgtest-dev
```

## 构建

```bash
cmake -S sdk-cpp -B build/sdk-cpp
cmake --build build/sdk-cpp
ctest --test-dir build/sdk-cpp
```

## 使用

```cpp
#include <iostream>
#include <memory>

#include "simple_abtest/sdk.h"

int main() {
  std::string error;
  auto client = simple_abtest::Client::Create(
      "http://127.0.0.1:8080", 1001, "your-token", 300, &error);
  if (!client) {
    std::cerr << error << std::endl;
    return 1;
  }

  auto result = client->AB("user-123", {{"country", "CN"}, {"platform", "ios"}});
  for (const auto& item : result.config) {
    std::cout << item.first << " => " << item.second << std::endl;
  }
  return 0;
}
```

`Client`对外暴露的是接口，包含：

- `AB(key, ctx)`：本地判定实验配置
- `Stamp()`：返回最近一次成功刷新时间戳
- `Refresh(error)`：外部主动触发刷新

`Create()`的`ttl`参数类型是整数，语义为秒。`Create()`总会先同步拉取一次初始化数据。当`ttl == 0`时不会启动后台自动刷新，只能通过`Refresh()`手动更新；当`ttl > 0`且小于等于60时会按60秒处理。对象析构时会自动释放后台资源，不需要单独调用关闭接口。

