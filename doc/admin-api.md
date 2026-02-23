## Admin HTTP API

This document describes the HTTP endpoints exposed by the admin service.

### Conventions

- **Content-Type**: `application/json` by default.
- **Optimistic Locking**: Mutating operations require a `version` field. The server checks this against the stored version.
  - On mismatch, returns `409 Conflict`.
  - On success, the resource (or parent) version is incremented.
- **Common Errors**: `400 Bad Request` (invalid input), `404 Not Found` (resource missing), `500 Internal Server Error`.

---

### POST `/api/app`

Create a new application.

**Request**

```json
{
  "name": "my-app",
  "description": "optional"
}
```

**Response (200 OK)**

```json
{
  "id": 1001,
  "name": "my-app",
  "version": 0,
  "description": "optional"
}
```

### GET `/api/app`

List all applications.

**Response (200 OK)**

```json
[
  {
    "id": 1001,
    "name": "my-app"
  }
]
```

### GET `/api/app/:id`

Get application details and its experiments.

**Response (200 OK)**

```json
{
  "id": 1001,
  "name": "my-app",
  "version": 3,
  "description": "optional",
  "experiment": [
    {
      "id": 2001,
      "status": 1,
      "name": "exp-A",
      "description": "optional"
    }
  ]
}
```

### PUT `/api/app/:id`

Update an application.

**Request**

```json
{
  "name": "new-name",
  "description": "new description",
  "version": 3
}
```

**Response (200 OK)**: Updated application object.

### DELETE `/api/app/:id`

Delete an application (must have no experiments).

**Request**

```json
{
  "version": 3
}
```

**Response (200 OK)**: Empty body; `403` if not empty.

---

### POST `/api/exp`

Create an experiment.

**Request**

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "name": "exp-A",
  "description": "optional"
}
```

**Response (200 OK)**

```json
{
  "id": 2001,
  "status": 0,
  "name": "exp-A",
  "version": 0,
  "description": "optional"
}
```

### GET `/api/exp/:id`

Get experiment details including layers.

**Response (200 OK)**

```json
{
  "id": 2001,
  "status": 1,
  "name": "exp-A",
  "description": "optional",
  "version": 3,
  "filter": [
    ...
  ],
  "layer": [
    {
      "id": 3001,
      "name": "layer-1"
    }
  ]
}
```

### PUT `/api/exp/:id`

Update experiment metadata and filter.

**Request**

```json
{
  "name": "new-name",
  "description": "new desc",
  "version": 3,
  "filter": [
    ...
  ]
}
```

**Response (200 OK)**: Updated experiment object.

### DELETE `/api/exp/:id`

Delete an experiment.

**Request**

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "version": 2
}
```

**Response (200 OK)**: Empty body.

### POST `/api/exp/:id/shuffle`

Regenerate experiment seed. Safe to call concurrently (no version check).

**Response (200 OK)**: Empty body.

### PUT `/api/exp/:id/status`

Toggle experiment status (`0`: stopped, `1`: active).

**Request**

```json
{
  "status": 1,
  "version": 3
}
```

**Response (200 OK)**: Empty body.

---

### POST `/api/lyr`

Create a layer (with a default segment).

**Request**

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "name": "layer-1"
}
```

**Response (200 OK)**: Created layer object.

### GET `/api/lyr/:id`

Get layer details including segments.

**Response (200 OK)**

```json
{
  "id": 3001,
  "name": "layer-1",
  "version": 2,
  "segment": [
    {
      "id": 4001,
      "begin": 0,
      "end": 50
    },
    {
      "id": 4002,
      "begin": 50,
      "end": 100
    }
  ]
}
```

### PUT `/api/lyr/:id`

Update layer metadata.

**Request**

```json
{
  "name": "new-name",
  "version": 2
}
```

**Response (200 OK)**: Updated layer object.

### DELETE `/api/lyr/:id`

Delete a layer.

**Request**

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "version": 2
}
```

**Response (200 OK)**: Empty body.

### POST `/api/lyr/:id/rebalance`

Rebalance layer segments. Segments must be contiguous and cover [0, 100).

**Request**

```json
{
  "version": 2,
  "segment": [
    {
      "id": 4001,
      "begin": 0,
      "end": 40
    },
    {
      "id": 4002,
      "begin": 40,
      "end": 100
    }
  ]
}
```

**Response (200 OK)**: Empty body.

---

### POST `/api/seg`

Create a new segment (initially `[100, 100)`, with a default group).

**Request**

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2
}
```

**Response (200 OK)**

```json
{
  "id": 4003,
  "begin": 100,
  "end": 100,
  "version": 0
}
```

### GET `/api/seg/:id`

Get segment details including groups.

**Response (200 OK)**

```json
{
  "id": 4001,
  "begin": 0,
  "end": 50,
  "version": 1,
  "group": [
    {
      "id": 5001,
      "share": 500,
      "name": "DEFAULT",
      "is_default": true
    },
    {
      "id": 5002,
      "share": 500,
      "name": "B"
    }
  ]
}
```

### DELETE `/api/seg/:id`

Delete a segment (must be empty range `begin == end`).

**Request**

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2,
  "version": 1
}
```

**Response (200 OK)**: Empty body.

### POST `/api/seg/:id/shuffle`

Regenerate segment seed. No version check.

**Response (200 OK)**: Empty body.

### POST `/api/seg/:id/rebalance`

Adjust traffic share between default and target group.

**Request**

```json
{
  "version": 1,
  "grp_id": 5002,
  "share": 300
}
```

**Response (200 OK)**: Empty body.

---

### POST `/api/grp`

Create a group (share 0).

**Request**

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "name": "variant-A"
}
```

**Response (200 OK)**: Created group object.

### GET `/api/grp/:id`

Get group details.

**Response (200 OK)**

```json
{
  "id": 5002,
  "share": 300,
  "name": "variant-A",
  "is_default": false,
  "version": 1,
  "cfg_id": 7001,
  "force_hit": [
    "u1",
    "u2"
  ],
  "config": "{...}"
}
```

### PUT `/api/grp/:id`

Update metadata, force-hit keys, and config ID.

**Request**

```json
{
  "name": "variant-A",
  "version": 1,
  "cfg_id": 7001,
  "force_hit": [
    "u1",
    "u2"
  ]
}
```

**Response (200 OK)**: Updated group object.

### DELETE `/api/grp/:id`

Delete a non-default group (must have `share == 0`).

**Request**

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "version": 1
}
```

**Response (200 OK)**: Empty body.

### GET `/api/grp/:id/cfg`

List historical configs.

**Query**: `begin` (timestamp, optional).

**Response (200 OK)**

```json
[
  {
    "id": 7001,
    "config": "{...}"
  },
  ...
]
```

### POST `/api/grp/:id/cfg`

Create a new config. Body is raw config content.

**Request**: Arbitrary content (e.g., JSON).

**Response (200 OK)**: 
```json
{ 
  "id": 7003 
}
```
