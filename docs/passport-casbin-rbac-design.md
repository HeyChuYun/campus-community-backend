# Passport Casbin RBAC Design

## Overview

`passport-service` now owns both authentication and authorization.

Authorization is implemented with Casbin:

- user identity comes from `access_token`
- account status comes from `users`
- role / permission enforcement comes from Casbin policy

`school-service` is not part of RBAC.

## Storage

Platform user data still stays in `users`.

Casbin policy data uses `casbin_rule`:

- `p` rules: role -> permission
- `g` rules: user -> role

SQL file:

- [004_create_casbin_rule.sql](/E:/project/hfjj/backend/deploy/sql/004_create_casbin_rule.sql)

## Model

Casbin model used in `passport-service`:

- request: `sub, obj, act`
- policy: `sub, obj, act`
- grouping: `g = _, _`
- matcher: `g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act`

Permission code mapping rule:

- `content.post.create` -> `obj=content.post`, `act=create`
- `admin.user.role.assign` -> `obj=admin.user.role`, `act=assign`

## Default policy

Built in roles:

- `user`
- `admin`

Built in permissions are seeded at startup by `passport-service`.

Examples:

- `user` gets `content.post.create`, `content.comment.create`, `message.send`
- `admin` gets all common user permissions plus `admin.user.role.assign`, `admin.user.ban`

## Flow

### VerifyAccessToken

Only validates token and loads current user.

### Authorize

Authorization result is:

`valid access token + allowed account status + casbin enforce == allow`

Order:

1. validate access token
2. load platform user
3. reject `BLOCKED` / `DEACTIVATED`
4. reject write permissions for `READ_ONLY`
5. call Casbin `Enforce`

### GetMyAuthorization

Returns:

- current user
- current roles from Casbin `GetRolesForUser`
- implicit permissions from Casbin `GetImplicitPermissionsForUser`

### AssignUserRoles

Uses Casbin grouping policies to replace target user roles.

Operator identity is derived from `operator_access_token`, not from a raw `user_id` parameter.

## Bootstrap

When a new user registers:

1. bind school account
2. assign Casbin role `user`
3. if no `admin` exists yet, assign `admin` to this first user

## Current RPCs

RBAC related RPCs in `passport.proto`:

- `Authorize`
- `GetMyAuthorization`
- `AssignUserRoles`
- `ListUserRoles`
- `ListRoles`
- `ListPermissions`

## Note

If you later expose role management to HTTP admin APIs, the gateway should pass the current administrator access token directly to this RPC.
