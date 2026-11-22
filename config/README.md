# Config Directory

`config/shared.local.yaml` stores shared local infrastructure config.

`config/services/` stores the committed local debug business config for each service.

Current model:

- `local + file`: merge `shared.local.yaml` + `services/*.yaml`
- `local + etcd`: merge `/hfjj/local/shared` + `/hfjj/local/services/*`
- `prod + etcd`: merge `/hfjj/prod/shared` + `/hfjj/prod/services/*`

This directory no longer uses:

- `dynamic.yml`
- `dynamic.example.yml`
- `rendered/`

If a config value is environment-specific:

- keep the local debug value in `shared.local.yaml` or `config/services/*.yaml`
- keep the production value in etcd
