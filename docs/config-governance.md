# Config Governance

## Runtime Sources

The project supports two runtime config sources only:

- `file`: merge `config/shared.local.yaml` with `config/services/<service>.yaml`
- `etcd`: merge shared config and service config from etcd

## Production

Production is standardized to:

- `APP_ENV=prod`
- `CONFIG_SOURCE=etcd`
- `ETCD_ENDPOINTS=<host:port[,host:port]>`
- `CONFIG_NAMESPACE=/hfjj`

Recommended etcd keys:

- `/hfjj/<env>/shared`
- `/hfjj/<env>/services/gateway`
- `/hfjj/<env>/services/passport`
- `/hfjj/<env>/services/school`
- `/hfjj/<env>/services/relation`
- `/hfjj/<env>/services/content`
- `/hfjj/<env>/services/im`
- `/hfjj/<env>/services/captcha-solver`
- `/hfjj/<env>/services/update`

## Local

### Local File

```dotenv
APP_ENV=local
CONFIG_SOURCE=file
CONFIG_DIR=config
```

### Local Etcd

```dotenv
APP_ENV=local
CONFIG_SOURCE=etcd
ETCD_ENDPOINTS=127.0.0.1:2379
CONFIG_NAMESPACE=/hfjj
```

## Rules

- Do not reintroduce `.env` as a required local runtime dependency.
- Do not reintroduce `dynamic.yml`, `dynamic.example.yml`, or rendered config directories.
- Keep local shared config in `config/shared.local.yaml`.
- Keep local service configs under `config/services/`.
- Keep production runtime config in etcd.
