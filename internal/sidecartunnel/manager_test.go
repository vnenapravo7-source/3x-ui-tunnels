package sidecartunnel

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestPrepareCSQTTSyncsEveryClientCredential(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	enabled := true
	inst := Instance{
		ID:       41,
		Protocol: model.CSQTT,
		Port:     54789,
		Settings: Settings{Clients: []Client{
			{Email: "alice", Password: "alice-secret", Enable: &enabled},
			{Email: "bob", Password: "bob-secret", Enable: &enabled},
		}},
	}

	// A fresh server is bootstrapped through passwords.json.
	if err := prepareCSQTT(inst); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(stateDir(inst), "passwords.json"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		MainPassword string                    `json:"main_password"`
		Passwords    map[string]map[string]any `json:"passwords"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.MainPassword != "alice-secret" || len(legacy.Passwords) != 2 || legacy.Passwords["bob-secret"]["name"] != "bob" {
		t.Fatalf("fresh CSQTT credentials were not kept per client: %#v", legacy)
	}

	// Once CSQTT has migrated to SQLite, reconciliation must retain the same
	// one-password-per-panel-client relationship.
	dbPath := filepath.Join(stateDir(inst), "csqtt.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE meta(key TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE passwords(password TEXT PRIMARY KEY, expires_at INTEGER, name TEXT, vk_hashes TEXT,
			dtls_port INTEGER, wg_port INTEGER, local_port INTEGER);`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := prepareCSQTT(inst); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM passwords WHERE password IN (?, ?)", "alice-secret", "bob-secret").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("SQLite contains %d panel client credentials, want 2", count)
	}
}

func TestFingerprintIgnoresGeneratedCupsCode(t *testing.T) {
	inst := Instance{
		ID:       7,
		Protocol: model.OpenFlux,
		Settings: Settings{Clients: []Client{{Password: strings.Repeat("01", 32)}}},
	}
	before := inst.fingerprint()
	inst.Settings.CupsCode = "runtime-room-code"
	if after := inst.fingerprint(); after != before {
		t.Fatalf("generated Cups room changed process fingerprint: %s != %s", after, before)
	}
}

func TestValidCupsCode(t *testing.T) {
	if !validCupsCode("WyJyb29tLWlkIl0") {
		t.Fatal("valid room list was rejected")
	}
	if validCupsCode("not-a-room-list") {
		t.Fatal("invalid room code was accepted")
	}
}

func TestOpenFluxContextMatchesCorePriorityRule(t *testing.T) {
	transports := []Transport{
		{Type: "mailru", URL: "https://cloud.mail.ru/public/first", Priority: 25},
		{Type: "yandex", URL: "https://docs.yandex.ru/edit/d/high", Priority: 75},
		{Type: "direct", URL: "203.0.113.7:443", Priority: 100},
	}
	if got := openFluxContext("", transports, ""); got != transports[1].URL {
		t.Fatalf("context = %q, want highest-priority document URL", got)
	}
	if got := openFluxContext("explicit", transports, ""); got != "explicit" {
		t.Fatalf("explicit context = %q", got)
	}
	if got := openFluxContext("", []Transport{{Type: "cupsonline"}}, "rooms"); got != "" {
		t.Fatalf("Cups room list must not become context, got %q", got)
	}
}

func TestEnsureWDTTKeysAreValidAndStable(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWDTTKeys(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "wg-keys.dat")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(first)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected four WDTT keys, got %d", len(lines))
	}
	for _, line := range lines {
		key, err := base64.StdEncoding.DecodeString(line)
		if err != nil || len(key) != 32 {
			t.Fatalf("invalid WDTT key: %v", err)
		}
	}
	if err := ensureWDTTKeys(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("WDTT keys changed on restart")
	}
}
