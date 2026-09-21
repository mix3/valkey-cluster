package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/creack/pty"
)

// 1. CLI Parameter Definitions (github.com/alecthomas/kong)
type CLI struct {
	IP              string `env:"IP" default:"" help:"IP address for inter-node communication and cluster creation (auto-detected if empty, defaults to 0.0.0.0 on macOS)"`
	InitialPort     int    `short:"p" env:"INITIAL_PORT" default:"7000" help:"Starting port number for standard instances"`
	Masters         int    `short:"m" env:"MASTERS" default:"3" help:"Number of master nodes"`
	SlavesPerMaster int    `short:"s" env:"SLAVES_PER_MASTER" default:"1" help:"Number of slave nodes per master"`
	Standalone      bool   `short:"n" env:"STANDALONE" default:"false" help:"Enable creating standalone nodes"`
	StandaloneCount int    `env:"STANDALONE_COUNT" default:"2" help:"Number of standalone nodes"`
	Sentinel        bool   `short:"S" env:"SENTINEL" help:"Enable Sentinel mode"`
	SentinelPort    *int   `help:"Starting port number for Sentinel (defaults to master port - 2000 if not specified)"`
	Password        string `short:"a" env:"PASSWORD" help:"Authentication password"`
	Bind            string `env:"BIND_ADDRESS" default:"0.0.0.0" help:"IP address to bind"`
	DataDir         string `env:"DATA_DIR" default:"/valkey-data" help:"Root path for data directories"`
	ModuleDir       string `env:"MODULE_DIR" default:"/usr/lib/valkey" help:"Directory path to automatically search for .so modules"`
	TLSPort         int    `env:"TLS_PORT" default:"0" help:"Starting TLS port number (enables TLS mode if specified)"`
	TLSCertFile     string `env:"TLS_CERT_FILE" help:"Path to TLS certificate file (.crt)"`
	TLSKeyFile      string `env:"TLS_KEY_FILE" help:"Path to TLS private key file (.key)"`
	TLSCACertFile   string `name:"tls-ca-cert-file" env:"TLS_CA_CERT_FILE" help:"Path to TLS CA certificate file (.crt)"`
	TLSAuhtClients  string `env:"TLS_AUTH_CLIENTS" default:"no" enum:"yes,no" help:""`
}

type NodeRole string

const (
	RoleMaster     NodeRole = "Master"
	RoleSlave      NodeRole = "Slave"
	RoleSentinel   NodeRole = "Sentinel"
	RoleStandalone NodeRole = "Standalone"
)

type NodeConfig struct {
	Role       NodeRole
	Port       int
	MasterPort int
	Dir        string
}

var ansiColors = []string{
	"\033[36m", // Cyan
	"\033[32m", // Green
	"\033[33m", // Yellow
	"\033[35m", // Magenta
	"\033[34m", // Blue
	"\033[96m", // Bright Cyan
	"\033[92m", // Bright Green
	"\033[93m", // Bright Yellow
	"\033[95m", // Bright Magenta
}

const colorReset = "\033[0m"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[Error] %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("valkey-orchestrator"),
		kong.Description("Orchestrator to launch and manage multiple local Valkey / Redis nodes seamlessly."),
	)
	return runInternal(ctx, &cli)
}

func runInternal(ctx context.Context, cli *CLI) error {
	// Validate parameter ranges and topology configurations
	if cli.Masters < 0 || cli.SlavesPerMaster < 0 || cli.StandaloneCount < 0 {
		return fmt.Errorf("invalid argument: node counts (-m, -s, --standalone-count) cannot be negative")
	}

	if cli.Standalone && cli.StandaloneCount <= 0 {
		return fmt.Errorf("invalid standalone configuration: --standalone (-n) is enabled but --standalone-count is %d (must be > 0)", cli.StandaloneCount)
	}

	if cli.Masters <= 0 && !cli.Standalone {
		return fmt.Errorf("no nodes to launch: please specify at least one master (-m > 0) or enable standalone mode (-n)")
	}

	// Convert paths to absolute paths
	absCert := toAbs(cli.TLSCertFile)
	absKey := toAbs(cli.TLSKeyFile)
	absCACert := toAbs(cli.TLSCACertFile)
	absDataDir := toAbs(cli.DataDir)

	// Module search
	var modules []string
	if cli.ModuleDir != "" {
		absModuleDir := toAbs(cli.ModuleDir)
		modules = findModules(absModuleDir)
	}

	// Determine target IP for cluster formation and Sentinel monitoring
	clusterTargetIP := cli.IP
	if clusterTargetIP == "" {
		if runtime.GOOS == "darwin" {
			clusterTargetIP = "0.0.0.0"
		} else {
			detectedIP, err := getLocalIP()
			if err != nil {
				fmt.Printf("[Error] Failed to detect local IP: %v\n", err)
				os.Exit(1)
			}
			clusterTargetIP = detectedIP
		}
	}
	fmt.Printf("[Orchestrator] Target IP for node interaction: %s\n", clusterTargetIP)

	// Plan topology and determine directories
	nodes := planTopology(cli, absDataDir)

	isTLS := cli.TLSPort > 0
	isCluster := !cli.Sentinel && cli.Masters > 0
	isSentinel := cli.Sentinel && cli.Masters > 0

	hasExistingClusterConfig := checkExistingClusterConfig(nodes)

	type RunningProcess struct {
		Cmd  *exec.Cmd
		Pty  *os.File
		Node NodeConfig
	}
	var runningProcesses []*RunningProcess
	var mu sync.Mutex

	colorIdx := 0

	fmt.Println("[Orchestrator] Preparing to start nodes...")

	for _, node := range nodes {
		// Create isolated data directory
		if err := os.MkdirAll(node.Dir, 0755); err != nil {
			fmt.Printf("[Error] Failed to create directory %s: %v\n", node.Dir, err)
			continue
		}

		// Replace stale IP in configuration files
		if isCluster {
			replaceIPInFile(filepath.Join(node.Dir, fmt.Sprintf("nodes_%d.conf", node.Port)), clusterTargetIP)
		} else if node.Role == RoleSentinel {
			replaceIPInFile(filepath.Join(node.Dir, fmt.Sprintf("sentinel_%d.conf", node.Port)), clusterTargetIP)
		}

		var bin string
		var args []string

		if node.Role == RoleSentinel {
			bin = "valkey-sentinel"
			confPath := filepath.Join(node.Dir, fmt.Sprintf("sentinel_%d.conf", node.Port))

			createSentinelConfig(confPath, node, clusterTargetIP, cli)
			args = append(args, confPath)
		} else {
			bin = "valkey-server"

			args = append(args, "--bind", cli.Bind)

			if isTLS {
				args = append(args, "--port", "0", "--tls-port", strconv.Itoa(node.Port))
				args = append(args, "--tls-cert-file", absCert, "--tls-key-file", absKey, "--tls-ca-cert-file", absCACert)
				args = append(args, "--tls-auth-clients", cli.TLSAuhtClients)
				if isCluster {
					args = append(args,
						"--cluster-port", strconv.Itoa(node.Port+10000),
						"--tls-cluster", "yes",
						"--tls-replication", "yes",
					)
				}
			} else {
				args = append(args, "--port", strconv.Itoa(node.Port))
			}

			args = append(args, "--appendonly", "yes")
			if isCluster {
				args = append(args,
					"--cluster-enabled", "yes",
					"--cluster-config-file", fmt.Sprintf("nodes_%d.conf", node.Port),
					"--cluster-node-timeout", "5000",
					"--cluster-announce-ip", clusterTargetIP,
					"--cluster-announce-port", strconv.Itoa(node.Port),
					"--cluster-announce-bus-port", strconv.Itoa(node.Port+10000),
				)
			} else {
				args = append(args, "--cluster-enabled", "no")
			}

			if cli.Password != "" {
				args = append(args, "--requirepass", cli.Password, "--masterauth", cli.Password)
			}

			for _, mod := range modules {
				args = append(args, "--loadmodule", mod)
			}

			if isSentinel && node.Role == RoleSlave {
				args = append(args, "--replicaof", clusterTargetIP, strconv.Itoa(node.MasterPort))
			}
		}

		cmd := exec.Command(bin, args...)
		cmd.Dir = node.Dir // Isolate execution directory

		ptmx, err := pty.Start(cmd)
		if err != nil {
			fmt.Printf("[Error] Failed to start process (%s:%d): %v\n", bin, node.Port, err)
			continue
		}

		proc := &RunningProcess{Cmd: cmd, Pty: ptmx, Node: node}
		mu.Lock()
		runningProcesses = append(runningProcesses, proc)
		mu.Unlock()

		color := ansiColors[colorIdx%len(ansiColors)]
		colorIdx++

		roleShort := string(node.Role)
		if len(roleShort) > 6 {
			roleShort = roleShort[:6]
		}
		prefix := fmt.Sprintf("%s[%-6s-%d]%s ", color, roleShort, node.Port, colorReset)

		go func(r io.Reader, pfx string) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				fmt.Printf("%s%s\n", pfx, scanner.Text())
			}
		}(ptmx, prefix)
	}

	// Automatic cluster creation logic
	if isCluster && !hasExistingClusterConfig {
		go func() {
			fmt.Println("[Orchestrator] Waiting for cluster nodes to initialize (3 seconds)...")
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return // Signal received while waiting
			}

			fmt.Println("[Orchestrator] Executing cluster creation...")

			cliBin := "valkey-cli"
			var cliArgs []string

			if isTLS {
				cliArgs = append(cliArgs,
					"--tls",
					"--cert", absCert,
					"--key", absKey,
					"--cacert", absCACert,
					"--insecure",
				)
			}

			cliArgs = append(cliArgs, "--cluster", "create")

			if cli.Password != "" {
				cliArgs = append(cliArgs, "-a", cli.Password)
			}

			for _, n := range nodes {
				if n.Role == RoleMaster || n.Role == RoleSlave {
					cliArgs = append(cliArgs, fmt.Sprintf("%s:%d", clusterTargetIP, n.Port))
				}
			}

			cliArgs = append(cliArgs, "--cluster-replicas", strconv.Itoa(cli.SlavesPerMaster), "--cluster-yes")

			cmd := exec.CommandContext(ctx, cliBin, cliArgs...)

			out, err := cmd.CombinedOutput()
			if err != nil {
				if ctx.Err() == nil {
					fmt.Printf("[Error] Cluster creation failed: %v\nOutput:\n%s\n", err, string(out))
				}
			} else {
				fmt.Printf("[Orchestrator] Cluster created successfully!\n%s\n", string(out))
			}
		}()
	}

	// Block until signal is received via Context
	<-ctx.Done()
	fmt.Println("\n[Orchestrator] Termination signal received. Gracefully shutting down Valkey processes...")

	var wg sync.WaitGroup
	mu.Lock()
	for _, rp := range runningProcesses {
		wg.Add(1)
		go func(p *RunningProcess) {
			defer wg.Done()
			if p.Cmd.Process != nil {
				// Send SIGINT to request graceful shutdown (saving data to disk)
				_ = p.Cmd.Process.Signal(syscall.SIGINT)

				// Wait for process termination with timeout fallback
				done := make(chan error, 1)
				go func() {
					done <- p.Cmd.Wait()
				}()

				select {
				case <-done:
					// Process exited gracefully
				case <-time.After(5 * time.Second):
					// Force kill if process does not exit in time
					fmt.Printf("[Orchestrator] Process on port %d did not stop in time, killing...\n", p.Node.Port)
					_ = p.Cmd.Process.Kill()
					<-done
				}
			}
			// Close PTY only after the process has completely stopped
			p.Pty.Close()
		}(rp)
	}
	mu.Unlock()

	wg.Wait()
	fmt.Println("[Orchestrator] All Valkey processes stopped safely. Exiting.")

	return nil
}

// Topology planning logic
func planTopology(cli *CLI, baseDataDir string) []NodeConfig {
	var nodes []NodeConfig
	currPort := cli.InitialPort
	if cli.TLSPort > 0 {
		currPort = cli.TLSPort
	}

	// Assign Master nodes
	masters := make([]int, cli.Masters)
	for i := 0; i < cli.Masters; i++ {
		masters[i] = currPort
		nodes = append(nodes, NodeConfig{
			Role: RoleMaster,
			Port: currPort,
			Dir:  filepath.Join(baseDataDir, fmt.Sprintf("node_%d", currPort)),
		})
		currPort++
	}

	// Assign Slave nodes
	for i := 0; i < cli.Masters; i++ {
		for j := 0; j < cli.SlavesPerMaster; j++ {
			nodes = append(nodes, NodeConfig{
				Role:       RoleSlave,
				Port:       currPort,
				MasterPort: masters[i],
				Dir:        filepath.Join(baseDataDir, fmt.Sprintf("node_%d", currPort)),
			})
			currPort++
		}
	}

	// Assign Sentinel nodes
	if cli.Sentinel && cli.Masters > 0 {
		for i := 0; i < cli.Masters; i++ {
			var sentinelPort int
			if cli.SentinelPort != nil {
				sentinelPort = *cli.SentinelPort + i
			} else {
				sentinelPort = masters[i] - 2000
			}

			nodes = append(nodes, NodeConfig{
				Role:       RoleSentinel,
				Port:       sentinelPort,
				MasterPort: masters[i],
				Dir:        filepath.Join(baseDataDir, fmt.Sprintf("node_%d", sentinelPort)),
			})
		}
	}

	// Assign Standalone nodes
	if cli.Standalone {
		for i := 0; i < cli.StandaloneCount; i++ {
			nodes = append(nodes, NodeConfig{
				Role: RoleStandalone,
				Port: currPort,
				Dir:  filepath.Join(baseDataDir, fmt.Sprintf("node_%d", currPort)),
			})
			currPort++
		}
	}

	return nodes
}

func createSentinelConfig(path string, node NodeConfig, targetIP string, cli *CLI) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("port %d\n", node.Port))
	sb.WriteString(fmt.Sprintf("bind %s\n", cli.Bind))
	sb.WriteString(fmt.Sprintf("sentinel monitor mymaster-%d %s %d 1\n", node.MasterPort, targetIP, node.MasterPort))
	sb.WriteString(fmt.Sprintf("sentinel down-after-milliseconds mymaster-%d 3000\n", node.MasterPort))
	sb.WriteString(fmt.Sprintf("sentinel failover-timeout mymaster-%d 10000\n", node.MasterPort))

	if cli.Password != "" {
		sb.WriteString(fmt.Sprintf("sentinel auth-pass mymaster-%d %s\n", node.MasterPort, cli.Password))
	}

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

func replaceIPInFile(filePath string, newIP string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	ipRegex := regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)
	replaced := ipRegex.ReplaceAll(data, []byte(newIP))

	_ = os.WriteFile(filePath, replaced, 0644)
}

func checkExistingClusterConfig(nodes []NodeConfig) bool {
	for _, n := range nodes {
		confPath := filepath.Join(n.Dir, fmt.Sprintf("nodes_%d.conf", n.Port))
		if _, err := os.Stat(confPath); err == nil {
			return true
		}
	}
	return false
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "127.0.0.1", nil
}

func toAbs(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func findModules(dir string) []string {
	var modules []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".so") {
			modules = append(modules, path)
		}
		return nil
	})
	return modules
}
