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

**Errors**

- `400`: invalid JSON body or empty `key`.
- `404`: `appid` does not exist in engine memory.

### GET `/app/:id`

Get the whole experiment payload for one application.

**Path Params**

- `id` (uint32, required): Application ID.

**Response (200 OK)**

- `Content-Type: application/json`
- `Content-Encoding: gzip`
- Body: gzipped JSON, the uncompressed content is the app experiment array (`[]core.Experiment`).

Example:

```bash
curl -sS http://127.0.0.1:8080/app/1001 --output app.json.gz
gzip -dc app.json.gz | jq .
```

**Errors**

- `400`: invalid `id`.
- `404`: app not found.
