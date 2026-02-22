## Admin HTTP API

This document describes the HTTP endpoints exposed by the admin service.

### Versioning and optimistic locking

- Each mutable resource (`application`, `experiment`, `exp_layer`, `exp_segment`, `exp_group`)
  has a `version` field used for optimistic locking.
- Update and delete operations require the caller to send the current `version` value; the
  server only applies the change when the stored version matches the provided one.
- On successful writes, the affected resource (or its parent, when structure changes) has
  its version incremented by 1.
- Some helper operations, such as seed shuffle, intentionally do not change any `version`
  field and are safe to call concurrently (last value wins).

---

### POST `/api/app`

Create a new application.

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "name": "my-app",
  "description": "optional description"
}
```

- `name` (string, required): Application name.
- `description` (string, optional): Human readable description.

**Responses**

- `200 OK` with created application:

```json
{
  "id": 1001,
  "name": "my-app",
  "version": 0,
  "description": "optional description"
}
```

- `400 Bad Request` if body is invalid or `name` is empty.
- `500 Internal Server Error` on database errors.

### GET `/api/app`

List all applications.

- **Method**: `GET`

**Response**

- `200 OK`:

```json
[
  { "id": 1001, "name": "my-app" },
  { "id": 1002, "name": "another-app" }
]
```

### GET `/api/app/:id`

Get details of a single application and its experiments.

- **Method**: `GET`
- **Path Parameter**:
  - `id` (uint32): Application ID.

**Response**

- `200 OK`:

```json
{
  "id": 1001,
  "name": "my-app",
  "version": 3,
  "description": "optional description",
  "experiment": [
    { "id": 2001, "status": 1, "name": "exp-A" }
  ]
}
```

- `400 Bad Request` if `id` is not a valid integer.
- `404 Not Found` if the application does not exist.
- `500 Internal Server Error` on database errors.

### PUT `/api/app/:id`

Update an application (optimistic locking).

- **Method**: `PUT`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "name": "new-name",
  "description": "new description",
  "version": 3
}
```

- `version` (uint32, required): Current version for optimistic locking.

**Responses**

- `200 OK` with updated application (version incremented by 1).
- `400 Bad Request` if body is invalid or `name` is empty.
- `409 Conflict` if version does not match (concurrent modification).
- `500 Internal Server Error` on database errors.

### DELETE `/api/app/:id`

Delete an application (only when it has no experiments).

- **Method**: `DELETE`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "version": 3
}
```

**Responses**

- `204 No Content` on success (body empty).
- `400 Bad Request` if body is invalid.
- `403 Forbidden` if there are experiments under the application.
- `409 Conflict` if version does not match.
- `500 Internal Server Error` on database errors.

---

### POST `/api/exp`

Create a new experiment under an application.

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "id": 0,
  "status": 0,
  "name": "exp-A",
  "version": 0,
  "description": "optional",
  "filter": [
    { "op": 6, "dtype": 1, "key": "country", "s": "CN" }
  ]
}
```

- `app_id` (uint32, required): Owning application ID.
- `app_ver` (uint32, required): Application version for optimistic locking.
- `name` (string, required): Experiment name.
- `description` (string, optional).
- `filter` (array, optional): Expression nodes defining filter criteria.

The filter is validated by `engine/core.ParseExpr`.

**Responses**

- `200 OK` with created experiment:

```json
{
  "id": 2001,
  "status": 0,
  "name": "exp-A",
  "version": 0,
  "description": "optional",
  "filter": [ ... ]
}
```

- `400 Bad Request` if name is empty or filter JSON is illegal.
- `409 Conflict` if application version check fails.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning application `version` is incremented by 1.

### GET `/api/exp/:id`

Get full experiment details including layers.

- **Method**: `GET`
- **Path Parameter**:
  - `id` (uint32): Experiment ID.

**Response**

- `200 OK`:

```json
{
  "id": 2001,
  "status": 1,
  "name": "exp-A",
  "version": 3,
  "description": "optional",
  "filter": [ ... ],
  "layer": [
    { "id": 3001, "name": "layer-1" }
  ]
}
```

- `400 Bad Request` if `id` is invalid.
- `404 Not Found` if experiment does not exist.
- `500 Internal Server Error` on database errors or broken filter JSON.

### PUT `/api/exp/:id`

Update experiment metadata and filter (optimistic locking).

- **Method**: `PUT`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "name": "new-name",
  "description": "new description",
  "status": 0,
  "version": 3,
  "filter": [ ... ]
}
```

**Responses**

- `200 OK` with updated experiment. The `version` field in the response is the
  new value (typically the request `version + 1`).
- `400 Bad Request` if name is empty or filter is illegal.
- `404 Not Found` if experiment is deleted during update.
- `409 Conflict` if version does not match.
- `500 Internal Server Error` on database errors.

### DELETE `/api/exp/:id`

Delete an experiment.

- **Method**: `DELETE`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "version": 2
}
```

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid body.
- `409 Conflict` if version does not match.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning application `version` is incremented by 1.

### POST `/api/exp/:id/shuffle`

Regenerate the experiment seed.

- **Method**: `POST`

**Responses**

- `204 No Content` on success.
- `400 Bad Request` if `id` is invalid.
- `404 Not Found` if experiment does not exist.
- `500 Internal Server Error` on database errors.

This operation does not change any `version` field.

### PUT `/api/exp/:id/switch`

Toggle experiment status (optimistic locking).

- **Method**: `PUT`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "status": 1,
  "version": 3
}
```

- `status` (uint8, required): New status value (0 = stopped, 1 = active).
- `version` (uint32, required): Expected current experiment version. The write
  only succeeds when this matches the stored value.

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid input or unsupported status.
- `404 Not Found` if experiment does not exist.
- `409 Conflict` if version does not match (concurrent modification).
- `500 Internal Server Error` on database errors.

On success, the experiment `version` is incremented by 1.

---

### POST `/api/lyr`

Create a new layer under an experiment (with a default segment).

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "id": 0,
  "name": "layer-1",
  "version": 0,
  "description": "optional"
}
```

**Responses**

- `200 OK` with created layer (version 0).
- `400 Bad Request` if name is empty or body invalid.
- `409 Conflict` if experiment version check fails.
- `500 Internal Server Error` on database errors.

On success, the owning experiment `version` is incremented by 1.

### GET `/api/lyr/:id`

Get layer details including its segments.

- **Method**: `GET`

**Response**

- `200 OK`:

```json
{
  "id": 3001,
  "name": "layer-1",
  "version": 2,
  "description": "optional",
  "segment": [
    { "id": 4001, "begin": 0, "end": 50 },
    { "id": 4002, "begin": 50, "end": 100 }
  ]
}
```

### PUT `/api/lyr/:id`

Update layer metadata (optimistic locking).

- **Method**: `PUT`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "name": "new-layer-name",
  "description": "optional",
  "version": 2
}
```

**Responses**

- `200 OK` with updated layer (version incremented by 1).
- `400 Bad Request` if name is empty.
- `409 Conflict` if version does not match.
- `500 Internal Server Error` on database errors.

### DELETE `/api/lyr/:id`

Delete a layer.

- **Method**: `DELETE`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "version": 2
}
```

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid body.
- `409 Conflict` if version does not match or dependent data changed.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning experiment `version` is incremented by 1.

### POST `/api/lyr/:id/rebalance`

Rebalance layer coverage across segments.

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "version": 2,
  "segment": [
    { "id": 4001, "begin": 0, "end": 40 },
    { "id": 4002, "begin": 40, "end": 100 }
  ]
}
```

Validation rules:

- At least 2 segments.
- First segment must start at 0.
- Last segment must end at 100.
- All segments must be contiguous: `segment[i].begin == segment[i-1].end`.
- No duplicated segment IDs.

**Responses**

- `204 No Content` on success.
- `400 Bad Request` if validation fails.
- `409 Conflict` if current DB segment set does not match the provided list
  or versions conflict.
- `500 Internal Server Error` on database or transaction errors.

On success, the layer `version` is incremented by 1.

---

### POST `/api/seg`

Create a new segment under a layer (with a default group).

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2
}
```

The new segment initially covers `[100, 100)` and does not receive traffic
until rebalanced.

**Responses**

- `200 OK`:

```json
{
  "id": 4003,
  "begin": 100,
  "end": 100,
  "version": 0
}
```

-- `400 Bad Request` on invalid body.
-- `409 Conflict` if layer version check fails.
-- `500 Internal Server Error` on database or transaction errors.

On success, the owning layer `version` is incremented by 1.

### GET `/api/seg/:id`

Get segment details including its groups.

- **Method**: `GET`

**Response**

- `200 OK`:

```json
{
  "id": 4001,
  "begin": 0,
  "end": 50,
  "version": 1,
  "group": [
    { "id": 5001, "share": 500, "name": "DEFAULT", "is_default": true },
    { "id": 5002, "share": 500, "name": "B" }
  ]
}
```

### DELETE `/api/seg/:id`

Delete a segment.

- **Method**: `DELETE`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2,
  "version": 1
}
```

The segment can only be deleted when `range_begin == range_end`.

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid body.
- `409 Conflict` if version does not match or segment is not empty.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning layer `version` is incremented by 1.

### POST `/api/seg/:id/shuffle`

Regenerate the segment seed used for hash slotting.

- **Method**: `POST`

**Responses**

- `204 No Content` on success.
- `400 Bad Request` if `id` is invalid.
- `404 Not Found` if segment does not exist.
- `500 Internal Server Error` on database errors.

This operation does not change any `version` field.

### POST `/api/seg/:id/rebalance`

Rebalance traffic between the default group and a specific group.

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "version": 1,
  "grp_id": 5002,
  "share": 300
}
```

- `grp_id` (uint32, required): Target non-default group ID.
- `share` (uint32, required): Desired number of slots assigned to this group.

The remaining slots are kept in the default group. The sum of both shares must
match the current total; otherwise the request is rejected.

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid body.
- `403 Forbidden` if rebalance target is the default group or required share
  exceeds total available slots.
- `409 Conflict` if bitmap or version conflicts are detected.
- `500 Internal Server Error` on database or transaction errors.

On success, the segment `version` is incremented by 1.

---

### POST `/api/grp`

Create a new group under a segment.

- **Method**: `POST`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "id": 0,
  "share": 0,
  "name": "variant-A",
  "version": 0,
  "description": "optional",
  "force_hit": [],
  "config": ""
}
```

**Responses**

- `200 OK` with created group (share 0, version 0, config empty).
- `400 Bad Request` if name is empty.
- `409 Conflict` if segment version check fails.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning segment `version` is incremented by 1.

### GET `/api/grp/:id`

Get group details, including config and force-hit keys.

- **Method**: `GET`

**Response**

- `200 OK`:

```json
{
  "id": 5002,
  "share": 300,
  "name": "variant-A",
  "is_default": false,
  "version": 1,
  "cfg_id": 7001,
  "description": "optional",
  "force_hit": ["user-1", "user-2"],
  "config": "{...}"
}
```

### PUT `/api/grp/:id`

Update group metadata, force-hit keys and bound config ID.

- **Method**: `PUT`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "name": "variant-A",
  "description": "optional",
  "share": 300,
  "is_default": false,
  "version": 1,
  "cfg_id": 7001,
  "force_hit": ["user-1", "user-2"],
  "config": "{...ignored on update...}"
}
```

Only `name`, `description`, `force_hit` and `cfg_id` are updated; `share`
and `config` are managed by other APIs.

**Responses**

- `200 OK` with refreshed group detail (including the new `version`).
- `400 Bad Request` if name is empty or body invalid.
- `404 Not Found` if group does not exist.
- `409 Conflict` if version does not match.
- `500 Internal Server Error` on database errors.

### DELETE `/api/grp/:id`

Delete a non-default group with zero share.

- **Method**: `DELETE`
- **Content-Type**: `application/json`

**Request Body**

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "version": 1
}
```

Group must be non-default and have `share == 0`.

**Responses**

- `204 No Content` on success.
- `400 Bad Request` on invalid body.
- `409 Conflict` if version mismatch or constraints not satisfied.
- `500 Internal Server Error` on database or transaction errors.

On success, the owning segment `version` is incremented by 1.

### GET `/api/grp/:id/cfg`

List historical configs of a group since a given time.

- **Method**: `GET`
- **Query Parameters**:
  - `begin` (int64, optional): Unix timestamp (seconds) for the earliest
    `create_time` to include. Defaults to 0 if omitted.

**Response**

- `200 OK`:

```json
[
  { "id": 7001, "config": "{...}" },
  { "id": 7002, "config": "{...}" }
]
```

### POST `/api/grp/:id/cfg`

Create a new config record for a group.

- **Method**: `POST`
- **Content-Type**: arbitrary (raw body used as config string)

**Request Body**

The entire HTTP body is treated as config content, for example:

```json
{ "button_color": "red" }
```

**Responses**

- `200 OK`:

```json
{ "id": 7003 }
```

- `400 Bad Request` if body cannot be read.
- `500 Internal Server Error` on database errors.
