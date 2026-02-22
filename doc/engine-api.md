## Engine HTTP API

This document describes the HTTP endpoints exposed by the engine service.

---

### POST `/`

Main A/B testing decision API.

- **Description**: Returns experiment layer configurations and debug tags for a given app and key.
- **Method**: `POST`
- **Content-Type**: `application/json`

#### Request Body

```json
{
  "appid": 1001,
  "key": "user-unique-key",
  "context": {
    "country": "CN",
    "platform": "ios"
  }
}
```

- `appid` (uint32, required): Application identifier, used to select experiments.
- `key` (string, required): Stable key used for hashing into experiment buckets (for example user ID).
- `context` (object, optional): Arbitrary key-value pairs used by experiment filters.

#### Responses

- **200 OK**

```json
{
  "config": {
    "layerA": "config-id-or-json",
    "layerB": "another-config"
  },
  "tags": [
    "layerA:group1",
    "layerB:control"
  ]
}
```

- `config`: Map from layer name to experiment configuration content.
- `tags`: List of debug tags in the form `<layer>:<group>`.

- **400 Bad Request**

Returned when the request body cannot be parsed or `key` is empty.
