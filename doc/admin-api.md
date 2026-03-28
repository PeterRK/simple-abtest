## Admin HTTP API

This document describes the HTTP endpoints exposed by the `admin` service.

### Base and Content Type

- Base path: `/api`
- JSON endpoints use `Content-Type: application/json`
- `POST /api/grp/:id/cfg` accepts raw body content (not JSON schema constrained)

### Session Authentication

Protected endpoints require `HttpOnly` cookies:

- `SESSION_UID`: user id (uint32)
- `SESSION_TOKEN`: session token

Session is issued by:

- `POST /api/user`
- `POST /api/user/login`

Cookies are set with `HttpOnly`, `SameSite=Lax`, `Path=/`, and `Secure` on HTTPS requests.

If missing/invalid/expired session:

- `401 Unauthorized` (or `400 Bad Request` when `SESSION_UID` format is invalid)
- Exception: `PUT /api/user/:id` and `DELETE /api/user/:id` validate `password` instead of session.

### Privilege Levels

- `0`: no access
- `1`: read-only
- `2`: read-write
- `3`: admin

Rules in current implementation:

- `appCreate`: requires valid session; creator is granted `admin` on new app.
- `appUpdate` and `appDelete`: require `admin` on app.
- Experiment and descendants (`exp/lyr/seg/grp/cfg`):
  - `GET` requires `read-only`
  - non-`GET` requires `read-write`

### Common Behavior

- Optimistic locking: many mutating endpoints require `version`; mismatch returns `409 Conflict`.
- Name validation:
  - allowed characters: `A-Z`, `a-z`, `0-9`, `_`, `-`
  - invalid or oversized names return `400 Bad Request`
  - max length by resource:
    - user name: `64`
    - application name: `64`
    - experiment name: `32`
    - layer name: `32`
    - group name: `32`
- Common status codes:
  - `400 Bad Request`: invalid input
  - `401 Unauthorized`: missing/invalid session or invalid password credential
  - `403 Forbidden`: privilege denied or business rule denied
  - `404 Not Found`: resource missing (only on part of endpoints)
  - `409 Conflict`: version/business conflict
  - `500 Internal Server Error`

---

## User and Privilege

### POST `/api/user`

Create user and issue session.

Request:

```json
{
  "name": "alice",
  "password": "secret",
  "secret": "invite-code"
}
```

Response `200 OK`:

```json
{
  "uid": 1
}
```

Notes:

- `secret` is an optional registration invite code / predefined secret.
- `name` must match the common name validation rule, max length `64`.
- when the server enables a predefined secret, invalid or missing `secret` may return `401 Unauthorized`
- duplicate name -> `409 Conflict`

### POST `/api/user/login`

Login and issue session.

Request:

```json
{
  "name": "alice",
  "password": "secret"
}
```

Response `200 OK`:

```json
{
  "uid": 1
}
```

Notes:

- `name` must match the common name validation rule, max length `64`.
- invalid user/password -> `401 Unauthorized`

### PUT `/api/user/:id`

Update user password.

Permission:

- old password required

Request:

```json
{
  "password": "old-secret",
  "new_password": "new-secret"
}
```

Response: `200 OK` empty body.

### DELETE `/api/user/:id`

Delete user.

Permission:

- old password required

Request:

```json
{
  "password": "old-secret"
}
```

Response: `200 OK` empty body.

### GET `/api/app/:id/privilege`

Get granted users of an app.

Permission:

- app `admin`

Response `200 OK`:

```json
[
  {
    "name": "alice",
    "privilege": 2,
    "grantor": "owner"
  }
]
```

### POST `/api/app/:id/privilege`

Grant/revoke privilege of a user on an app.

Permission:

- app `admin`

Request:

```json
{
  "name": "alice",
  "privilege": 2
}
```

`privilege=0` means revoke.

Response: `200 OK` empty body.

Notes:

- `name` must match the common name validation rule, max length `64`.

---

## Application

### POST `/api/app`

Create application.

Permission: session required.

Request:

```json
{
  "name": "my-app",
  "description": "optional"
}
```

Response `200 OK`:

```json
{
  "id": 1001,
  "name": "my-app",
  "version": 0,
  "description": "optional"
}
```

Notes:

- `name` must match the common name validation rule, max length `64`.
- creator gets `admin` privilege on this app automatically.
- server still generates and stores one app-level secret in `application.access_token`, but it is no longer returned by create/list/detail APIs.

### GET `/api/app`

List applications accessible by current user.

Permission: session required.

Response `200 OK`:

```json
[
  {
    "id": 1001,
    "name": "my-app"
  }
]
```

### GET `/api/app/:id`

Get application detail and experiment summaries.

Permission: app `read-only`.

Response `200 OK`:

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
      "version": 3,
      "name": "exp-A",
      "description": "optional"
    }
  ]
}
```

### PUT `/api/app/:id`

Update application.

Permission: app `admin`.

Request:

```json
{
  "name": "new-name",
  "description": "new description",
  "version": 3
}
```

Response: `200 OK` empty body.

Notes:

- `name` must match the common name validation rule, max length `64`.
- current implementation updates `name` and `description` only.

### POST `/api/app/:id/token`

Issue one short-lived public token for engine access.

Permission: app `admin`.

Request:

```json
{
  "ttl_seconds": 300
}
```

Response `200 OK`:

```json
{
  "token": "<public-token>",
  "expire_at": "2026-03-28 10:30:00"
}
```

Notes:

- `name` must match the common name validation rule, max length `32`.

Notes:

- `ttl_seconds` must be positive.
- if the derived `expire_at` cannot fit into the token's uint32 unix-seconds field, server returns `400 Bad Request`.
- returned `token` is a short-lived public token signed from the app's stored `access_token`; it is not the stored secret itself.

### DELETE `/api/app/:id`

Delete application.

Permission: app `admin`.

Request:

```json
{
  "version": 3
}
```

Response: `200 OK` empty body.

Notes:

- `name` must match the common name validation rule, max length `32`.

Notes:

- if app still has experiments -> `403 Forbidden`

---

## Engine Proxy

### POST `/engine`

Proxy one online verification request through `admin` to `engine`.

Permission: valid session plus app `read-only`.

Request:

```json
{
  "appid": 1001,
  "key": "user-123",
  "context": {
    "country": "CN"
  }
}
```

Response `200 OK`:

```json
{
  "config": {
    "layerA": "..."
  },
  "tags": ["layerA:control"]
}
```

Notes:

- `name` must match the common name validation rule, max length `32`.

Notes:

- current implementation requires `appid` to be present in request body so admin can check app privilege before proxying.
- admin signs a temporary 60-second public token internally, then calls engine with that token.
- callers do not need to provide `ACCESS_TOKEN` when using this proxy endpoint.

---

## Experiment

### POST `/api/exp`

Create experiment (also creates default layer/segment/group).

Permission: app `read-write`.

Request:

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "name": "exp-A",
  "description": "optional"
}
```

Response `200 OK`:

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

Permission: experiment `read-only`.

Response `200 OK`:

```json
{
  "id": 2001,
  "app_id": 1001,
  "status": 1,
  "name": "exp-A",
  "description": "optional",
  "version": 3,
  "filter": [],
  "layer": [
    {
      "id": 3001,
      "name": "layer-1"
    }
  ]
}
```

### PUT `/api/exp/:id`

Permission: experiment `read-write`.

Request:

```json
{
  "name": "new-name",
  "description": "new desc",
  "version": 3,
  "filter": []
}
```

Response: `200 OK` empty body.

Notes:

- `name` must match the common name validation rule, max length `32`.

### DELETE `/api/exp/:id`

Permission: experiment `read-write`.

Request:

```json
{
  "app_id": 1001,
  "app_ver": 3,
  "version": 2
}
```

Response: `200 OK` empty body.

### POST `/api/exp/:id/shuffle`

Regenerate seed (no version check).

Permission: experiment `read-write`.

Response: `200 OK` empty body.

### PUT `/api/exp/:id/status`

Toggle status (`0` stopped, `1` active).

Permission: experiment `read-write`.

Request:

```json
{
  "status": 1,
  "version": 3
}
```

Response: `200 OK` empty body.

---

## Layer

### POST `/api/lyr`

Create layer.

Permission: experiment `read-write`.

Request:

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "name": "layer-1"
}
```

Response `200 OK`:

```json
{
  "id": 3001,
  "name": "layer-1",
  "version": 0
}
```

Notes:

- `name` must match the common name validation rule, max length `32`.

### GET `/api/lyr/:id`

Permission: layer `read-only`.

Response `200 OK`:

```json
{
  "id": 3001,
  "name": "layer-1",
  "version": 2,
  "segment": [
    {
      "id": 4001,
      "begin": 0,
      "end": 50,
      "version": 1
    }
  ]
}
```

### PUT `/api/lyr/:id`

Permission: layer `read-write`.

Request:

```json
{
  "name": "new-name",
  "version": 2
}
```

Response: `200 OK` empty body.

Notes:

- `name` must match the common name validation rule, max length `32`.

### DELETE `/api/lyr/:id`

Permission: layer `read-write`.

Request:

```json
{
  "exp_id": 2001,
  "exp_ver": 3,
  "version": 2
}
```

Response: `200 OK` empty body.

### POST `/api/lyr/:id/rebalance`

Rebalance layer segments; must be contiguous and cover `[0,100)`.

Permission: layer `read-write`.

Request:

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

Response: `200 OK` empty body.

---

## Segment

### POST `/api/seg`

Create segment (`[100,100)` initially, with default group).

Permission: layer `read-write`.

Request:

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2
}
```

Response `200 OK`:

```json
{
  "id": 4003,
  "begin": 100,
  "end": 100,
  "version": 0
}
```

### GET `/api/seg/:id`

Permission: segment `read-only`.

Response `200 OK`:

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
      "is_default": true,
      "version": 1
    }
  ]
}
```

### DELETE `/api/seg/:id`

Delete segment (requires `begin == end`).

Permission: segment `read-write`.

Request:

```json
{
  "lyr_id": 3001,
  "lyr_ver": 2,
  "version": 1
}
```

Response: `200 OK` empty body.

### POST `/api/seg/:id/shuffle`

Regenerate seed (no version check).

Permission: segment `read-write`.

Response: `200 OK` empty body.

### POST `/api/seg/:id/rebalance`

Adjust share between default group and one target group.

Permission: segment `read-write`.

Request:

```json
{
  "version": 1,
  "grp_id": 5002,
  "share": 300
}
```

Response: `200 OK` empty body.

---

## Group and Config

### POST `/api/grp`

Create group (initial share `0`).

Permission: segment `read-write`.

Request:

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "name": "variant-A"
}
```

Response `200 OK`:

```json
{
  "id": 5002,
  "name": "variant-A",
  "share": 0,
  "version": 0
}
```

### GET `/api/grp/:id`

Permission: group `read-only`.

Response `200 OK`:

```json
{
  "id": 5002,
  "share": 300,
  "name": "variant-A",
  "is_default": false,
  "version": 1,
  "cfg_id": 7001,
  "cfg_stamp": "2026-02-26 12:00:00",
  "force_hit": ["u1", "u2"],
  "config": "{...}"
}
```

### PUT `/api/grp/:id`

Permission: group `read-write`.

Request:

```json
{
  "name": "variant-A",
  "version": 1,
  "cfg_id": 7001,
  "force_hit": ["u1", "u2"]
}
```

Response: `200 OK` empty body.

### DELETE `/api/grp/:id`

Delete non-default group (requires `share == 0`).

Permission: group `read-write`.

Request:

```json
{
  "seg_id": 4001,
  "seg_ver": 1,
  "version": 1
}
```

Response: `200 OK` empty body.

### GET `/api/grp/:id/cfg`

List config history.

Permission: group `read-only`.

Query:

- `begin` (optional, unix timestamp seconds, default `0`)

Response `200 OK`:

```json
[
  {
    "id": 7001,
    "stamp": "2026-02-26 12:00:00"
  }
]
```

Notes:

- Returns at most **50** records, ordered by `id` descending (newest first).
- Use `begin` to filter by creation time; records with `stamp < begin` are excluded.

### POST `/api/grp/:id/cfg`

Create config record; request body is raw content.

Permission: group `read-write`.

Response `200 OK`:

```json
{
  "id": 7003,
  "stamp": "2026-02-26 12:30:00"
}
```

### GET `/api/grp/:gid/cfg/:cid`

Get one config content.

Permission: group `read-only`.

Response `200 OK`: raw content body.
