# simple-abtest C++ SDK

[中文](README.md) | [English](README-EN.md)

`sdk-cpp` is the C++ local-evaluation SDK for `simple-abtest`. It evaluates experiments from local snapshots inside your application process, with decision behavior aligned with the traffic allocation service.

## Features

- Synchronously pulls `GET /app/:id` during initialization
- Optionally refreshes experiment snapshots in the background
- Evaluates filter conditions and experiment decisions locally
- Returns configuration and hit tags for each layer

## Dependencies

- C++17
- `libcurl`
- `zlib`
- `protobuf` / `protoc`
- `gtest` (tests only)

Example for Ubuntu/Debian:

```bash
sudo apt-get install -y cmake g++ libcurl4-openssl-dev zlib1g-dev protobuf-compiler libprotobuf-dev libgtest-dev
```

## Build

```bash
cmake -S sdk-cpp -B build/sdk-cpp
cmake --build build/sdk-cpp
ctest --test-dir build/sdk-cpp
```

## Usage

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

## API

`Client` exposes the following interface:

- `AB(key, ctx)`: evaluates experiment config locally
- `Stamp()`: returns the timestamp of the most recent successful refresh
- `Refresh(error)`: actively triggers one refresh

`Create()` always performs one synchronous initial pull. The `ttl` parameter is an integer in seconds:

- when `ttl == 0`, automatic background refresh is disabled and updates must be triggered through `Refresh()`
- when `0 < ttl <= 60`, the SDK uses `60` seconds
- when `ttl > 60`, the SDK refreshes at the provided interval

The client releases its background resources during destruction, so no separate close call is required.
