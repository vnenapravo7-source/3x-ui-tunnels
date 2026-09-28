package sidecartunnel

import (
	"bufio"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type Client struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Enable     *bool  `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
}

type Transport struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

type Settings struct {
	Codec          string      `json:"codec"`
	Mode           string      `json:"mode"`
	Negotiate      bool        `json:"negotiate"`
	SessionContext string      `json:"sessionContextUrl"`
	Transports     []Transport `json:"transports"`
	WGPort         int         `json:"wgPort"`
	LocalPort      int         `json:"localPort"`
	Hashes         []string    `json:"hashes"`
	Clients        []Client    `json:"clients"`
	CupsCode       string      `json:"cupsCode,omitempty"`
}

type Instance struct {
	ID       int
	Protocol model.Protocol
	Listen   string
	Port     int
	Settings Settings
}

func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || (ib.Protocol != model.OpenFlux && ib.Protocol != model.WDTT && ib.Protocol != model.CSQTT) {
		return Instance{}, false
	}
	var settings Settings
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		return Instance{}, false
	}
	active := settings.Clients[:0]
	for _, client := range settings.Clients {
		if client.Enable != nil && !*client.Enable {
			continue
		}
		if strings.TrimSpace(client.Password) == "" {
			continue
		}
		active = append(active, client)
	}
	settings.Clients = active
	if len(active) == 0 {
		return Instance{}, false
	}
	return Instance{ID: ib.Id, Protocol: ib.Protocol, Listen: ib.Listen, Port: ib.Port, Settings: settings}, true
}

func (i Instance) fingerprint() string {
	// cupsCode is runtime output, not process configuration. Persisting a newly
	// created room must not make the reconciler restart OpenFlux and create it again.
	i.Settings.CupsCode = ""
	raw, _ := json.Marshal(i)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type managed struct {
	instance    Instance
	fingerprint string
	cmd         *exec.Cmd
	done        chan struct{}
	cancel      context.CancelFunc
}

type Manager struct {
	mu    sync.Mutex
	procs map[int]*managed
}

const (
	csqttInterface = "csqttxui"
	csqttAddress   = "10.66.68.1/24"
	csqttSubnet    = "10.66.68.0/24"
)

var singleton = &Manager{procs: make(map[int]*managed)}

func GetManager() *Manager { return singleton }

func binaryCandidates(protocol model.Protocol) []string {
	binDir := config.GetBinFolderPath()
	openFluxArch := runtime.GOARCH
	if openFluxArch == "arm" {
		openFluxArch = "armv7"
	}
	names := map[model.Protocol][]string{
		model.OpenFlux: {fmt.Sprintf("openflux-%s-%s", runtime.GOOS, openFluxArch), "openflux"},
		model.WDTT:     {fmt.Sprintf("wdtt-server-%s-%s", runtime.GOOS, runtime.GOARCH), "wdtt-server"},
		model.CSQTT:    {fmt.Sprintf("csqtt-%s-%s", runtime.GOOS, runtime.GOARCH), "csqtt"},
	}[protocol]
	var out []string
	for _, name := range names {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		out = append(out, filepath.Join(binDir, name))
	}
	for _, name := range names {
		out = append(out, filepath.Join("/usr/local/bin", name), filepath.Join("/usr/bin", name))
		if path, err := exec.LookPath(name); err == nil {
			out = append(out, path)
		}
	}
	return out
}

func binaryPath(protocol model.Protocol) (string, error) {
	for _, path := range binaryCandidates(protocol) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s server binary is not installed; run x-ui tunnels install %s", protocol, protocol)
}

func stateDir(inst Instance) string {
	return filepath.Join(config.GetBinFolderPath(), "tunnels", fmt.Sprintf("%s-%d", inst.Protocol, inst.ID))
}

func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".xui-tunnel-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func passwordMap(inst Instance) map[string]map[string]any {
	result := make(map[string]map[string]any, len(inst.Settings.Clients))
	for _, client := range inst.Settings.Clients {
		expires := int64(0)
		if client.ExpiryTime > 0 {
			expires = client.ExpiryTime / 1000
		}
		result[client.Password] = map[string]any{
			"device_id": "", "expires_at": expires, "down_bytes": 0, "up_bytes": 0,
			"label": client.Email, "name": client.Email,
			"vk_hashes": strings.Join(inst.Settings.Hashes, ","),
			"ports":     fmt.Sprintf("%d,%d,%d", inst.Port, inst.Settings.WGPort, inst.Settings.LocalPort),
			"dtls_port": inst.Port, "wg_port": inst.Settings.WGPort, "local_port": inst.Settings.LocalPort,
		}
	}
	return result
}

func prepareWDTT(inst Instance) error {
	dir := stateDir(inst)
	if err := ensureWDTTKeys(dir); err != nil {
		return err
	}
	path := filepath.Join(dir, "passwords.json")
	data := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &data)
	}
	data["main_password"] = inst.Settings.Clients[0].Password
	data["passwords"] = passwordMap(inst)
	if data["devices"] == nil {
		data["devices"] = map[string]any{}
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(path, raw)
}

// WDTT refuses to generate WireGuard keys when its access database already
// contains clients. We seed that database before launch, so create the two
// persistent keypairs first. Never replace a server's existing key file.
func ensureWDTTKeys(dir string) error {
	path := filepath.Join(dir, "wg-keys.dat")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("invalid WDTT key file %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	curve := ecdh.X25519()
	server, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	client, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	lines := []string{
		base64.StdEncoding.EncodeToString(server.Bytes()),
		base64.StdEncoding.EncodeToString(server.PublicKey().Bytes()),
		base64.StdEncoding.EncodeToString(client.Bytes()),
		base64.StdEncoding.EncodeToString(client.PublicKey().Bytes()),
	}
	return writePrivate(path, []byte(strings.Join(lines, "\n")+"\n"))
}

func prepareCSQTT(inst Instance) error {
	dir := stateDir(inst)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dbPath := filepath.Join(dir, "csqtt.db")
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		legacy := map[string]any{
			"main_password": inst.Settings.Clients[0].Password,
			"passwords":     passwordMap(inst), "devices": map[string]any{},
		}
		raw, marshalErr := json.MarshalIndent(legacy, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		return writePrivate(filepath.Join(dir, "passwords.json"), raw)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "INSERT INTO meta(key,value) VALUES('main_password',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", inst.Settings.Clients[0].Password); err != nil {
		return err
	}
	wanted := make([]string, 0, len(inst.Settings.Clients))
	for _, client := range inst.Settings.Clients {
		wanted = append(wanted, client.Password)
		expires := int64(0)
		if client.ExpiryTime > 0 {
			expires = client.ExpiryTime / 1000
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO passwords(password,expires_at,name,vk_hashes,dtls_port,wg_port,local_port)
			VALUES(?,?,?,?,?,?,?) ON CONFLICT(password) DO UPDATE SET expires_at=excluded.expires_at,name=excluded.name,
			vk_hashes=excluded.vk_hashes,dtls_port=excluded.dtls_port,wg_port=excluded.wg_port,local_port=excluded.local_port`,
			client.Password, expires, client.Email, strings.Join(inst.Settings.Hashes, ","), inst.Port, inst.Settings.WGPort, inst.Settings.LocalPort)
		if err != nil {
			return err
		}
	}
	if len(wanted) > 0 {
		marks := strings.TrimRight(strings.Repeat("?,", len(wanted)), ",")
		args := make([]any, len(wanted))
		for i := range wanted {
			args[i] = wanted[i]
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM passwords WHERE password NOT IN ("+marks+")", args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// The upstream CSQTT deployer opens the peer port and configures forwarding
// for its fixed TUN subnet. A managed sidecar does not run that deployer, so
// apply the same minimal, idempotent rules before starting the process.
func prepareCSQTTNetwork(port int) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("CSQTT requires /dev/net/tun: %w", err)
	}
	// The pinned, patched CSQTT build uses a separate interface and subnet so
	// an existing SWG-CSQTT on csqtt1 can remain untouched.
	ctx := context.Background()
	if exec.CommandContext(ctx, "ip", "link", "show", "dev", csqttInterface).Run() == nil {
		return fmt.Errorf("CSQTT cannot start: TUN %s is already present; identify its owner before removing anything", csqttInterface)
	}
	if output, err := exec.CommandContext(ctx, "ip", "-4", "route", "show", csqttSubnet).Output(); err == nil && strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("CSQTT cannot start: subnet %s already has a route", csqttSubnet)
	}
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644); err != nil {
		return fmt.Errorf("enable IPv4 forwarding for CSQTT: %w", err)
	}
	output, err := exec.CommandContext(context.Background(), "ip", "-4", "route", "get", "1.1.1.1").Output()
	if err != nil {
		return fmt.Errorf("detect CSQTT WAN interface: %w", err)
	}
	words := strings.Fields(string(output))
	wan := ""
	for index, word := range words {
		if word == "dev" && index+1 < len(words) {
			wan = words[index+1]
			break
		}
	}
	if wan == "" {
		return errors.New("CSQTT WAN interface not found")
	}
	rules := []struct {
		table string
		chain string
		args  []string
	}{
		{"filter", "INPUT", []string{"-p", "udp", "--dport", strconv.Itoa(port), "-m", "comment", "--comment", "3XUI_CSQTT", "-j", "ACCEPT"}},
		{"filter", "INPUT", []string{"-i", csqttInterface, "-s", csqttSubnet, "-m", "comment", "--comment", "3XUI_CSQTT", "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-i", csqttInterface, "-m", "comment", "--comment", "3XUI_CSQTT", "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-o", csqttInterface, "-m", "comment", "--comment", "3XUI_CSQTT", "-j", "ACCEPT"}},
		{"nat", "POSTROUTING", []string{"-s", csqttSubnet, "-o", wan, "-m", "comment", "--comment", "3XUI_CSQTT", "-j", "MASQUERADE"}},
	}
	for _, rule := range rules {
		if err := ensureIPTablesRule(rule.table, rule.chain, rule.args); err != nil {
			return fmt.Errorf("configure CSQTT firewall: %w", err)
		}
	}
	return nil
}

func ensureIPTablesRule(table, chain string, args []string) error {
	check := append([]string{"-w", "2", "-t", table, "-C", chain}, args...)
	if exec.CommandContext(context.Background(), "iptables", check...).Run() == nil {
		return nil
	}
	insert := append([]string{"-w", "2", "-t", table, "-I", chain, "1"}, args...)
	if output, err := exec.CommandContext(context.Background(), "iptables", insert...).CombinedOutput(); err != nil {
		return fmt.Errorf("iptables %s/%s: %w: %s", table, chain, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func prepareWDTTNetwork(port int) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("WDTT requires /dev/net/tun: %w", err)
	}
	args := []string{"-p", "udp", "--dport", strconv.Itoa(port), "-m", "comment", "--comment", "3XUI_WDTT", "-j", "ACCEPT"}
	return ensureIPTablesRule("filter", "INPUT", args)
}

func prepareOpenFlux(inst Instance) error {
	secret := strings.TrimSpace(inst.Settings.Clients[0].Password)
	decoded, err := hex.DecodeString(secret)
	if err != nil || len(decoded) != 32 {
		return errors.New("OpenFlux client key must contain exactly 64 hexadecimal characters")
	}
	return writePrivate(filepath.Join(stateDir(inst), "secret.key"), []byte(secret+"\n"))
}

func openFluxContext(explicit string, transports []Transport, cupsCode string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	best := -1
	for i, transport := range transports {
		url := strings.TrimSpace(transport.URL)
		if transport.Type == "cupsonline" && url == "" {
			url = strings.TrimSpace(cupsCode)
		}
		if url == "" || url == "http://#" {
			continue
		}
		switch transport.Type {
		case "cupsonline", "direct", "oneme":
			continue
		}
		if best < 0 || transport.Priority > transports[best].Priority {
			best = i
		}
	}
	if best >= 0 {
		return strings.TrimSpace(transports[best].URL)
	}
	return ""
}

func commandFor(inst Instance) (*exec.Cmd, error) {
	bin, err := binaryPath(inst.Protocol)
	if err != nil {
		return nil, err
	}
	dir := stateDir(inst)
	listen := inst.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	switch inst.Protocol {
	case model.WDTT:
		if err := prepareWDTT(inst); err != nil {
			return nil, err
		}
		if err := prepareWDTTNetwork(inst.Port); err != nil {
			return nil, err
		}
		return exec.CommandContext(context.Background(), bin, "--listen", fmt.Sprintf("%s:%d", listen, inst.Port), "--wg-port", strconv.Itoa(inst.Settings.WGPort), "--config-dir", dir), nil
	case model.CSQTT:
		if err := prepareCSQTTNetwork(inst.Port); err != nil {
			return nil, err
		}
		if err := prepareCSQTT(inst); err != nil {
			return nil, err
		}
		return exec.CommandContext(context.Background(), bin,
			"--listen", fmt.Sprintf("%s:%d", listen, inst.Port),
			"--config-dir", dir,
			"--iface", csqttInterface,
			"--tun-addr", csqttAddress,
			"--no-web"), nil
	case model.OpenFlux:
		if len(inst.Settings.Clients) != 1 {
			return nil, errors.New("OpenFlux inbound requires exactly one client")
		}
		if err := prepareOpenFlux(inst); err != nil {
			return nil, err
		}
		mode := inst.Settings.Mode
		if mode == "" {
			mode = "l4"
		}
		codec := inst.Settings.Codec
		if codec == "" {
			codec = "batched"
		}
		if len(inst.Settings.Transports) > 1 && codec != "batched" {
			return nil, errors.New("OpenFlux multi-transport requires batched codec")
		}
		if len(inst.Settings.Transports) == 1 && codec != "batched" &&
			(inst.Settings.Negotiate || inst.Settings.Transports[0].Type == "direct") &&
			inst.Settings.Transports[0].Type != "cupsonline" {
			return nil, errors.New("OpenFlux negotiated session requires batched codec")
		}
		args := []string{"--role=exit", "--mode=" + mode, "--codec=" + codec, "--encryption-key-file=" + filepath.Join(dir, "secret.key")}
		if len(inst.Settings.Transports) > 0 {
			parts := make([]string, 0, len(inst.Settings.Transports))
			seen := map[string]bool{}
			contextURL := openFluxContext(inst.Settings.SessionContext, inst.Settings.Transports, inst.Settings.CupsCode)
			for _, transport := range inst.Settings.Transports {
				if seen[transport.Type] {
					return nil, fmt.Errorf("duplicate OpenFlux transport %s is not supported by the lightweight runner", transport.Type)
				}
				seen[transport.Type] = true
				parts = append(parts, fmt.Sprintf("%s:%d", transport.Type, transport.Priority))
				url := strings.TrimSpace(transport.URL)
				if transport.Type == "cupsonline" && url == "" {
					url = inst.Settings.CupsCode
				}
				if url != "" && len(inst.Settings.Transports) > 1 {
					args = append(args, "--"+transport.Type+"-url="+url)
				}
			}
			if len(inst.Settings.Transports) == 1 {
				transport := inst.Settings.Transports[0]
				args = append(args, "--transport="+transport.Type)
				transportURL := strings.TrimSpace(transport.URL)
				if transport.Type == "cupsonline" && transportURL == "" {
					transportURL = strings.TrimSpace(inst.Settings.CupsCode)
				}
				if transport.Type != "direct" && transportURL != "" {
					args = append(args, "--url="+transportURL)
				}
				if (inst.Settings.Negotiate || transport.Type == "direct") && transport.Type != "cupsonline" {
					args = append(args, "--negotiate")
				}
			} else {
				args = append(args, "--transports="+strings.Join(parts, ","), "--negotiate")
			}
			if contextURL != "" {
				args = append(args, "--session-context="+contextURL)
			}
			if seen["direct"] {
				args = append(args, fmt.Sprintf("--direct-listen=%s:%d", listen, inst.Port))
			}
		} else {
			return nil, errors.New("OpenFlux requires at least one transport")
		}
		if !seenDirectOnly(inst.Settings.Transports) {
			args = append(args, "--cookie-store="+filepath.Join(dir, "cookies.json"))
		}
		return exec.CommandContext(context.Background(), bin, args...), nil
	default:
		return nil, fmt.Errorf("unsupported sidecar protocol %s", inst.Protocol)
	}
}

func seenDirectOnly(transports []Transport) bool {
	return len(transports) == 1 && transports[0].Type == "direct"
}

func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst.Protocol == model.CSQTT {
		for id, other := range m.procs {
			if id != inst.ID && other.instance.Protocol == model.CSQTT && other.cmd != nil {
				return fmt.Errorf("3x-ui manages one CSQTT instance on %s (already managed by inbound %d)", csqttInterface, id)
			}
		}
	}
	fingerprint := inst.fingerprint()
	if current := m.procs[inst.ID]; current != nil && current.fingerprint == fingerprint && current.cmd != nil {
		return nil
	}
	m.removeLocked(inst.ID)
	cmd, err := commandFor(inst)
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	ctx, cancel := context.WithCancel(context.Background())
	managed := &managed{instance: inst, fingerprint: fingerprint, cmd: cmd, done: make(chan struct{}), cancel: cancel}
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	m.procs[inst.ID] = managed
	go m.capture(ctx, managed, stdout)
	go m.wait(managed)
	return nil
}

func (m *Manager) capture(ctx context.Context, proc *managed, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	expectCode := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "=== COPY THIS TO CLIENT ===") {
			expectCode = true
			continue
		}
		if expectCode && line != "" {
			expectCode = false
			if validCupsCode(line) {
				m.persistCupsCode(proc, line)
				continue // Room codes are client credentials; never write them to logs.
			}
		}
		if strings.Contains(line, "generated web password:") || strings.Contains(line, "generated main password:") {
			line = "[generated CSQTT password redacted]"
		}
		if line != "" {
			logger.Infof("%s[%d]: %s", proc.instance.Protocol, proc.instance.ID, line)
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func validCupsCode(value string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	var rooms []string
	return json.Unmarshal(raw, &rooms) == nil && len(rooms) > 0
}

func (m *Manager) persistCupsCode(proc *managed, code string) {
	m.mu.Lock()
	current := m.procs[proc.instance.ID]
	if current != proc || current.instance.Settings.CupsCode == code {
		m.mu.Unlock()
		return
	}
	firstRoom := current.instance.Settings.CupsCode == ""
	current.instance.Settings.CupsCode = code
	m.mu.Unlock()
	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, proc.instance.ID).Error; err != nil {
		return
	}
	var settings map[string]any
	if json.Unmarshal([]byte(inbound.Settings), &settings) != nil {
		return
	}
	settings["cupsCode"] = code
	raw, err := json.Marshal(settings)
	if err == nil {
		_ = database.GetDB().Model(&model.Inbound{}).Where("id = ?", proc.instance.ID).UpdateColumn("settings", string(raw)).Error
	}
	hasCups := false
	for _, transport := range proc.instance.Settings.Transports {
		if transport.Type == "cupsonline" {
			hasCups = true
			break
		}
	}
	if firstRoom && hasCups {
		go func() {
			time.Sleep(time.Second)
			m.mu.Lock()
			if m.procs[proc.instance.ID] != proc {
				m.mu.Unlock()
				return
			}
			inst := proc.instance
			m.removeLocked(inst.ID)
			m.mu.Unlock()
			if err := m.Ensure(inst); err != nil {
				logger.Warningf("OpenFlux[%d] restart after Cups room creation failed: %v", inst.ID, err)
			}
		}()
	}
}

func (m *Manager) wait(proc *managed) {
	err := proc.cmd.Wait()
	close(proc.done)
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.procs[proc.instance.ID]
	if current != proc {
		return
	}
	current.cmd = nil
	if err != nil {
		logger.Warningf("%s[%d] exited: %v", proc.instance.Protocol, proc.instance.ID, err)
	}
}

func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(id)
}

func (m *Manager) removeLocked(id int) {
	proc := m.procs[id]
	if proc == nil {
		return
	}
	delete(m.procs, id)
	proc.cancel()
	if proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = proc.cmd.Process.Kill()
	} else {
		_ = proc.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-proc.done:
			return
		case <-time.After(3 * time.Second):
			_ = proc.cmd.Process.Kill()
		}
	}
}

func (m *Manager) Reconcile(instances []Instance) {
	sort.Slice(instances, func(a, b int) bool { return instances[a].ID < instances[b].ID })
	wanted := make(map[int]bool, len(instances))
	for _, inst := range instances {
		wanted[inst.ID] = true
		if err := m.Ensure(inst); err != nil {
			logger.Warningf("%s[%d] start failed: %v", inst.Protocol, inst.ID, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.procs {
		if !wanted[id] {
			m.removeLocked(id)
		}
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int, 0, len(m.procs))
	for id := range m.procs {
		ids = append(ids, id)
	}
	for _, id := range ids {
		m.removeLocked(id)
	}
}

// RestartProtocol restarts only the managed processes for protocol. It is
// used after an atomic sidecar-binary update so unrelated tunnels and Xray
// keep running. Instances that previously exited are retried as well.
func (m *Manager) RestartProtocol(protocol model.Protocol) error {
	m.mu.Lock()
	instances := make([]Instance, 0)
	for id, proc := range m.procs {
		if proc.instance.Protocol != protocol {
			continue
		}
		instances = append(instances, proc.instance)
		m.removeLocked(id)
	}
	m.mu.Unlock()

	sort.Slice(instances, func(i, j int) bool { return instances[i].ID < instances[j].ID })
	var errs []error
	started := make([]int, 0, len(instances))
	for _, inst := range instances {
		if err := m.Ensure(inst); err != nil {
			errs = append(errs, fmt.Errorf("%s[%d]: %w", protocol, inst.ID, err))
			continue
		}
		started = append(started, inst.ID)
	}
	if len(started) > 0 {
		time.Sleep(750 * time.Millisecond)
		m.mu.Lock()
		for _, id := range started {
			if proc := m.procs[id]; proc == nil || proc.cmd == nil {
				errs = append(errs, fmt.Errorf("%s[%d]: process exited during startup", protocol, id))
			}
		}
		m.mu.Unlock()
	}
	return errors.Join(errs...)
}
