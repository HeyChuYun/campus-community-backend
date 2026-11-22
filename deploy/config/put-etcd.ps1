$Endpoints = $env:ETCD_ENDPOINTS
if ([string]::IsNullOrWhiteSpace($Endpoints)) { $Endpoints = "127.0.0.1:2379" }

$AppEnv = $env:APP_ENV
if ([string]::IsNullOrWhiteSpace($AppEnv)) { $AppEnv = "prod" }

$Namespace = $env:CONFIG_NAMESPACE
if ([string]::IsNullOrWhiteSpace($Namespace)) { $Namespace = "/hfjj" }

$Root = $Namespace.TrimEnd("/") + "/" + $AppEnv

function Put-EtcdYaml([string]$Key, [string]$Path) {
  $fullPath = (Resolve-Path $Path).Path
  # 多行 YAML 不能直接当成命令行参数传给 etcdctl。
  # PowerShell 管道对多行文本的处理也不稳定，这里统一改成 cmd 文件重定向，确保原始内容完整写入 etcd。
  cmd /c "type ""$fullPath"" | etcdctl --endpoints=""$Endpoints"" put ""$Key"""
}

Put-EtcdYaml "$Root/shared" ".\deploy\config\shared.prod.yaml"
Put-EtcdYaml "$Root/services/gateway" ".\deploy\config\services\gateway.yaml"
Put-EtcdYaml "$Root/services/passport" ".\deploy\config\services\passport.yaml"
Put-EtcdYaml "$Root/services/school" ".\deploy\config\services\school.yaml"
Put-EtcdYaml "$Root/services/relation" ".\deploy\config\services\relation.yaml"
Put-EtcdYaml "$Root/services/content" ".\deploy\config\services\content.yaml"
Put-EtcdYaml "$Root/services/im" ".\deploy\config\services\im.yaml"
Put-EtcdYaml "$Root/services/update" ".\deploy\config\services\update.yaml"
Put-EtcdYaml "$Root/services/captcha-solver" ".\deploy\config\services\captcha-solver.yaml"

