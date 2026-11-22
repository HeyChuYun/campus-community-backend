# Deploy

`deploy/` contains production deployment assets only.

## Files

- `compose/docker-compose.app.yml`: production application service orchestration
- `compose/docker-compose.build.yml`: optional image build override
- `config/`: production config source files and etcd publish scripts
- `env/app.env.example`: production env template
- `env/app.env`: production env used by compose
- `sql/*.sql`: database migration SQL

## Production Config Source

Production uses etcd as the runtime config source.

Compose should inject:

- `APP_ENV=prod`
- `CONFIG_SOURCE=etcd`
- `ETCD_ENDPOINTS`
- `CONFIG_NAMESPACE`

Production config files should be maintained under `deploy/config/` and published to etcd before deployment.

## Start

```bash
make prod-pull
make prod-up
```
