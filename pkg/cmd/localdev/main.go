package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type service struct {
	Name string
	Dir  string
}

var services = []service{
	{Name: "captcha-solver-service", Dir: filepath.Join("service", "captcha-solver-service")},
	{Name: "relation-service", Dir: filepath.Join("service", "relation-service")},
	{Name: "school-service", Dir: filepath.Join("service", "school-service")},
	{Name: "passport-service", Dir: filepath.Join("service", "passport-service")},
	{Name: "content-service", Dir: filepath.Join("service", "content-service")},
	{Name: "im-service", Dir: filepath.Join("service", "im-service")},
	{Name: "update-service", Dir: filepath.Join("service", "update-service")},
	{Name: "gateway", Dir: filepath.Join("service", "gateway")},
}

const (
	configSourceFile = "file"
	configSourceEtcd = "etcd"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: go run ./pkg/cmd/localdev <file|file-dev|file-windows|etcd|etcd-dev|etcd-windows|stop> [service]")
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("%v", err)
	}

	switch os.Args[1] {
	case "file":
		if err := up(root, configSourceFile, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "file-dev":
		if err := dev(root, configSourceFile, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "file-windows":
		if err := windows(root, configSourceFile, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "etcd":
		if err := up(root, configSourceEtcd, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "etcd-dev":
		if err := dev(root, configSourceEtcd, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "etcd-windows":
		if err := windows(root, configSourceEtcd, serviceArg()); err != nil {
			fatalf("%v", err)
		}
	case "stop":
		if err := stop(root); err != nil {
			fatalf("%v", err)
		}
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func serviceArg() string {
	if len(os.Args) < 3 {
		return ""
	}
	return os.Args[2]
}

func up(root string, source string, onlyService string) error {
	env, configPath, _, binRoot, err := prepare(root, source)
	if err != nil {
		return err
	}

	selected, err := selectServices(onlyService)
	if err != nil {
		return err
	}

	for _, svc := range selected {
		serviceDir := filepath.Join(root, svc.Dir)
		binPath := filepath.Join(binRoot, binaryName(svc.Name))
		if err := buildService(serviceDir, binPath); err != nil {
			return err
		}

		pid, err := startService(svc, serviceDir, binPath, configPath, filepath.Join(root, "logs", "local"), env)
		if err != nil {
			return err
		}
		if err := appendPID(filepath.Join(root, "logs", "local", "pids.txt"), pid, svc.Name); err != nil {
			return err
		}
		fmt.Printf("started %s pid=%d config=%s\n", svc.Name, pid, configPath)
		time.Sleep(time.Second)
	}

	fmt.Printf("local services are starting. gateway: http://127.0.0.1:%s\n", getenvFrom(env, "GATEWAY_PORT", "10000"))
	fmt.Printf("config: %s\n", configPath)
	fmt.Printf("logs: %s\n", filepath.Join(root, "logs", "local"))
	fmt.Printf("pids: %s\n", filepath.Join(root, "logs", "local", "pids.txt"))
	return nil
}

func dev(root string, source string, onlyService string) error {
	env, configPath, _, binRoot, err := prepare(root, source)
	if err != nil {
		return err
	}

	selected, err := selectServices(onlyService)
	if err != nil {
		return err
	}

	var commands []*exec.Cmd
	var wg sync.WaitGroup
	stopAll := func() {
		for _, cmd := range commands {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		stopAll()
	}()

	for _, svc := range selected {
		serviceDir := filepath.Join(root, svc.Dir)
		binPath := filepath.Join(binRoot, binaryName(svc.Name))
		if err := buildService(serviceDir, binPath); err != nil {
			stopAll()
			return err
		}

		cmd, err := startForegroundService(svc, serviceDir, binPath, configPath, env, &wg)
		if err != nil {
			stopAll()
			return err
		}
		commands = append(commands, cmd)
		fmt.Printf("started %s pid=%d config=%s\n", svc.Name, cmd.Process.Pid, configPath)
		time.Sleep(time.Second)
	}

	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] exited: %v\n", filepath.Base(cmd.Path), err)
		}
	}
	stopAll()
	wg.Wait()
	return nil
}

func windows(root string, source string, onlyService string) error {
	if runtime.GOOS != "windows" {
		return errors.New("windows mode only supports Windows")
	}

	env, configPath, logRoot, binRoot, err := prepare(root, source)
	if err != nil {
		return err
	}

	selected, err := selectServices(onlyService)
	if err != nil {
		return err
	}

	pidFile := filepath.Join(logRoot, "pids.txt")
	for _, svc := range selected {
		serviceDir := filepath.Join(root, svc.Dir)
		binPath := filepath.Join(binRoot, binaryName(svc.Name))
		if err := buildService(serviceDir, binPath); err != nil {
			return err
		}

		pid, err := startWindowsServiceWindow(root, svc, serviceDir, binPath, configPath, env)
		if err != nil {
			return err
		}
		if err := appendPID(pidFile, pid, svc.Name); err != nil {
			return err
		}
		fmt.Printf("started %s window pid=%d config=%s\n", svc.Name, pid, configPath)
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// selectServices 支持 relation、relation-service、gateway 等多种写法。
// 不传服务名时，默认启动全部服务。
func selectServices(name string) ([]service, error) {
	name = normalizeSelectedService(name)
	if name == "" {
		return services, nil
	}

	for _, svc := range services {
		if normalizeSelectedService(svc.Name) == name {
			return []service{svc}, nil
		}
	}
	return nil, fmt.Errorf("unknown service %q", name)
}

func normalizeSelectedService(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimSuffix(name, "-service")
	switch name {
	case "captcha":
		return "captcha-solver"
	default:
		return name
	}
}

func prepare(root string, source string) ([]string, string, string, string, error) {
	env := os.Environ()
	env = applyLocalDefaults(env, source)

	logRoot := filepath.Join(root, "logs", "local")
	binRoot := filepath.Join(root, ".local", "bin")
	if err := os.MkdirAll(logRoot, 0o755); err != nil {
		return nil, "", "", "", err
	}
	if err := os.MkdirAll(binRoot, 0o755); err != nil {
		return nil, "", "", "", err
	}

	pidFile := filepath.Join(logRoot, "pids.txt")
	if err := os.WriteFile(pidFile, nil, 0o644); err != nil {
		return nil, "", "", "", err
	}

	// 统一把配置目录传给各服务，再由 appconfig 自动合并共享配置和服务配置。
	return env, filepath.Join(root, "config"), logRoot, binRoot, nil
}

func stop(root string) error {
	pidFile := filepath.Join(root, "logs", "local", "pids.txt")
	file, err := os.Open(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = proc.Kill()
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(pidFile)
}

func buildService(serviceDir, binPath string) error {
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = serviceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startService(svc service, serviceDir, binPath, configPath, logRoot string, env []string) (int, error) {
	outLog, err := os.Create(filepath.Join(logRoot, svc.Name+".out.log"))
	if err != nil {
		return 0, err
	}
	errLog, err := os.Create(filepath.Join(logRoot, svc.Name+".err.log"))
	if err != nil {
		_ = outLog.Close()
		return 0, err
	}

	cmd := exec.Command(binPath, "-f", configPath)
	cmd.Dir = serviceDir
	cmd.Env = withServiceEnv(env, svc.Name)
	cmd.Stdout = outLog
	cmd.Stderr = errLog
	if err := cmd.Start(); err != nil {
		_ = outLog.Close()
		_ = errLog.Close()
		return 0, err
	}
	_ = outLog.Close()
	_ = errLog.Close()
	return cmd.Process.Pid, nil
}

func startForegroundService(svc service, serviceDir, binPath, configPath string, env []string, wg *sync.WaitGroup) (*exec.Cmd, error) {
	cmd := exec.Command(binPath, "-f", configPath)
	cmd.Dir = serviceDir
	cmd.Env = withServiceEnv(env, svc.Name)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	wg.Add(2)
	go streamLines(wg, svc.Name, "out", stdout, os.Stdout)
	go streamLines(wg, svc.Name, "err", stderr, os.Stderr)
	return cmd, nil
}

func startWindowsServiceWindow(root string, svc service, serviceDir, binPath, configPath string, env []string) (int, error) {
	scriptDir := filepath.Join(root, ".local", "windows")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return 0, err
	}
	scriptPath := filepath.Join(scriptDir, svc.Name+".ps1")
	script := windowsServiceScript(svc, serviceDir, binPath, configPath, withServiceEnv(env, svc.Name))
	if err := writeUTF8BOM(scriptPath, script); err != nil {
		return 0, err
	}

	startCommand := fmt.Sprintf(
		`$p = Start-Process -FilePath powershell.exe -WorkingDirectory '%s' -ArgumentList @('-NoExit','-ExecutionPolicy','Bypass','-File','%s') -PassThru; $p.Id`,
		escapePowerShellSingleQuote(serviceDir),
		escapePowerShellSingleQuote(scriptPath),
	)
	output, err := exec.Command("powershell.exe", "-NoProfile", "-Command", startCommand).Output()
	if err != nil {
		return 0, err
	}
	pidText := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func windowsServiceScript(svc service, serviceDir, binPath, configPath string, env []string) string {
	var builder strings.Builder
	builder.WriteString("$Host.UI.RawUI.WindowTitle = '")
	builder.WriteString(escapePowerShellSingleQuote(svc.Name))
	builder.WriteString("'\n")
	builder.WriteString("Set-Location -LiteralPath '")
	builder.WriteString(escapePowerShellSingleQuote(serviceDir))
	builder.WriteString("'\n")
	builder.WriteString(windowsEnvAssignments(env))
	builder.WriteString("& '")
	builder.WriteString(escapePowerShellSingleQuote(binPath))
	builder.WriteString("' -f '")
	builder.WriteString(escapePowerShellSingleQuote(configPath))
	builder.WriteString("'\n")
	builder.WriteString("Read-Host\n")
	return builder.String()
}

func withServiceEnv(env []string, serviceName string) []string {
	env = setEnv(env, "SERVICE_NAME", serviceName)
	return env
}

func windowsEnvAssignments(env []string) string {
	var builder strings.Builder
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		builder.WriteString("Set-Item -LiteralPath 'Env:\\")
		builder.WriteString(escapePowerShellSingleQuote(key))
		builder.WriteString("' -Value '")
		builder.WriteString(escapePowerShellSingleQuote(value))
		builder.WriteString("'\n")
	}
	return builder.String()
}

func writeUTF8BOM(path string, text string) error {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...)
	return os.WriteFile(path, data, 0o644)
}

func escapePowerShellSingleQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func streamLines(wg *sync.WaitGroup, serviceName string, streamName string, reader io.Reader, writer io.Writer) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Fprintf(writer, "[%s:%s] %s\n", serviceName, streamName, scanner.Text())
	}
}

func appendPID(path string, pid int, name string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "%d %s\n", pid, name)
	return err
}

func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func setEnv(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func getenvFrom(env []string, key string, fallback string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return fallback
}

func applyLocalDefaults(env []string, source string) []string {
	// 本地启动默认使用 local 环境；是否从文件或 etcd 读取，由 CONFIG_SOURCE 决定。
	if getenvFrom(env, "APP_ENV", "") == "" {
		env = setEnv(env, "APP_ENV", "local")
	}
	if getenvFrom(env, "CONFIG_SOURCE", "") == "" {
		env = setEnv(env, "CONFIG_SOURCE", source)
	}
	if getenvFrom(env, "ETCD_ENDPOINTS", "") == "" {
		env = setEnv(env, "ETCD_ENDPOINTS", "127.0.0.1:2379")
	}
	if getenvFrom(env, "CONFIG_NAMESPACE", "") == "" {
		env = setEnv(env, "CONFIG_NAMESPACE", "/hfjj")
	}
	if getenvFrom(env, "CONFIG_DIR", "") == "" {
		env = setEnv(env, "CONFIG_DIR", "config")
	}

	// 为了兼容仓库里尚未迁移的旧脚本，这里保留旧变量的镜像值。
	if getenvFrom(env, "MODE", "") == "" {
		env = setEnv(env, "MODE", getenvFrom(env, "APP_ENV", "local"))
	}
	if getenvFrom(env, "ETCD_NAMESPACE", "") == "" {
		env = setEnv(env, "ETCD_NAMESPACE", strings.TrimRight(getenvFrom(env, "CONFIG_NAMESPACE", "/hfjj"), "/")+"/"+getenvFrom(env, "APP_ENV", "local"))
	}
	return env
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
