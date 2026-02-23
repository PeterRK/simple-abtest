## Engine HTTP API

### POST `/`

Main A/B testing decision API.

**Request**

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

- `appid` (uint32, required): Application ID.
- `key` (string, required): Stable hashing key (e.g. user ID).
- `context` (object, optional): Variables for experiment filters.

**Response (200 OK)**

```json
{
  "config": { 
    "layerA": "config-id-or-json",
    "layerB": "..."
  },
  "tags": [ "layerA:group1", "layerB:control" ]
}
```

**Errors**: `400` on invalid body or empty key.
