#!/usr/bin/env bash
set -euo pipefail

ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-127.0.0.1:2379}"
APP_ENV="${APP_ENV:-prod}"
CONFIG_NAMESPACE="${CONFIG_NAMESPACE:-/hfjj}"
ETCD_ROOT="${CONFIG_NAMESPACE%/}/${APP_ENV}"

etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/shared" < ./deploy/config/shared.prod.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/gateway" < ./deploy/config/services/gateway.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/passport" < ./deploy/config/services/passport.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/school" < ./deploy/config/services/school.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/relation" < ./deploy/config/services/relation.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/content" < ./deploy/config/services/content.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/im" < ./deploy/config/services/im.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/update" < ./deploy/config/services/update.yaml
etcdctl --endpoints="${ETCD_ENDPOINTS}" put "${ETCD_ROOT}/services/captcha-solver" < ./deploy/config/services/captcha-solver.yaml

