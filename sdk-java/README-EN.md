# simple-abtest Java SDK

[中文](README.md) | [English](README-EN.md)

`sdk-java` is the Java local-evaluation SDK for `simple-abtest`. It evaluates experiments from local snapshots inside your application process, with decision behavior aligned with the traffic allocation service.

## Features

- Synchronously pulls `GET /app/:id` during initialization
- Optionally refreshes experiment snapshots in the background
- Evaluates filter conditions and experiment decisions locally
- Returns configuration and hit tags for each layer

The project uses the standard Maven layout and Gson for JSON parsing.

## Build And Test

```bash
cd sdk-java
mvn test
```

## Usage

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

## API

- `Client.create(address, appid, accessToken, ttlSeconds)`: creates the client and immediately pulls one experiment snapshot
- `client.ab(key, ctx)`: returns config and tags from the current local snapshot
- `client.stamp()`: returns the Unix timestamp of the most recent successful refresh
- `client.refresh()`: actively refreshes the local snapshot once
- `client.close()`: stops the background refresh thread

`ttlSeconds` is measured in seconds:

- when `ttlSeconds == 0`, automatic background refresh is disabled
- when `0 < ttlSeconds <= 60`, the SDK uses `60` seconds
- when `ttlSeconds > 60`, the SDK refreshes at the provided interval
