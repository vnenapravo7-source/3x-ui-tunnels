package sidecartunnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	openFluxReleaseBase = "https://github.com/vnenapravo7-source/3x-ui-tunnels/releases/download/sidecars-edge"
	maxOpenFluxBinary   = 64 << 20
	maxChecksumFile     = 4096
)

var openFluxUpdateHTTPClient = &http.Client{Timeout: 75 * time.Second}

// OpenFluxUpdateInfo is deliberately checksum-based: the rolling sidecar can
// carry upstream fixes without forcing a full 3x-ui release, and the digest is
// an exact, verifiable version of the installed server executable.
type OpenFluxUpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Installed       bool   `json:"installed"`
}

func openFluxAsset(goos, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("OpenFlux server updates are supported on Linux only")
	}
	switch goarch {
	case "amd64", "arm64":
		return "openflux-linux-" + goarch, nil
	case "arm":
		return "openflux-linux-armv7", nil
	default:
		return "", fmt.Errorf("unsupported OpenFlux architecture %s", goarch)
	}
}

func openFluxTargetPath(asset string) string {
	return filepath.Join(config.GetBinFolderPath(), asset)
}

func shortDigest(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func checksumVersion(digest string) string {
	if digest == "" {
		return "не установлен"
	}
	return "sha256:" + shortDigest(digest)
}

func parseChecksum(raw []byte, asset string) (string, error) {
	fields := strings.Fields(string(raw))
	if len(fields) < 1 {
		return "", errors.New("empty OpenFlux checksum response")
	}
	digest := strings.ToLower(fields[0])
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		return "", errors.New("invalid OpenFlux SHA-256 checksum")
	}
	if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") != asset {
		return "", fmt.Errorf("OpenFlux checksum belongs to %q, not %q", fields[1], asset)
	}
	return digest, nil
}

func fetchSmall(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := openFluxUpdateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("download %s exceeds %d bytes", url, limit)
	}
	return raw, nil
}

func fileSHA256(path string) (string, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", false, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), true, nil
}

func latestOpenFluxDigest(ctx context.Context, asset string) (string, error) {
	raw, err := fetchSmall(ctx, openFluxReleaseBase+"/"+asset+".sha256", maxChecksumFile)
	if err != nil {
		return "", err
	}
	return parseChecksum(raw, asset)
}

func GetOpenFluxUpdateInfo(ctx context.Context) (*OpenFluxUpdateInfo, error) {
	asset, err := openFluxAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	latest, err := latestOpenFluxDigest(ctx, asset)
	if err != nil {
		return nil, err
	}
	current, installed, err := fileSHA256(openFluxTargetPath(asset))
	if err != nil {
		return nil, err
	}
	return &OpenFluxUpdateInfo{
		CurrentVersion:  checksumVersion(current),
		LatestVersion:   checksumVersion(latest),
		UpdateAvailable: !installed || current != latest,
		Installed:       installed,
	}, nil
}

func downloadOpenFlux(ctx context.Context, url, target, expected string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := openFluxUpdateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download OpenFlux: HTTP %d", resp.StatusCode)
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, maxOpenFluxBinary+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > maxOpenFluxBinary {
		return errors.New("OpenFlux binary exceeds the size limit")
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if got != expected {
		return fmt.Errorf("OpenFlux checksum mismatch: got %s, want %s", got, expected)
	}
	return os.Chmod(target, 0o755)
}

// UpdateOpenFlux atomically replaces only the OpenFlux server executable.
// Existing inbounds, keys, cookies and generated Cups room codes stay intact.
// A one-generation backup is restored if the new binary cannot be restarted.
func UpdateOpenFlux(ctx context.Context) (*OpenFluxUpdateInfo, error) {
	asset, err := openFluxAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	expected, err := latestOpenFluxDigest(ctx, asset)
	if err != nil {
		return nil, err
	}
	target := openFluxTargetPath(asset)
	current, installed, err := fileSHA256(target)
	if err != nil {
		return nil, err
	}
	if installed && current == expected {
		return GetOpenFluxUpdateInfo(ctx)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".openflux-update-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(tmpPath); err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)
	if err := downloadOpenFlux(ctx, openFluxReleaseBase+"/"+asset, tmpPath, expected); err != nil {
		return nil, err
	}

	backup := target + ".previous"
	if installed {
		_ = os.Remove(backup)
		if err := os.Rename(target, backup); err != nil {
			return nil, fmt.Errorf("backup current OpenFlux: %w", err)
		}
	}
	if err := os.Rename(tmpPath, target); err != nil {
		if installed {
			_ = os.Rename(backup, target)
		}
		return nil, fmt.Errorf("install OpenFlux: %w", err)
	}
	if err := GetManager().RestartProtocol(model.OpenFlux); err != nil {
		_ = os.Remove(target)
		if installed {
			if rollbackErr := os.Rename(backup, target); rollbackErr != nil {
				return nil, errors.Join(fmt.Errorf("restart updated OpenFlux: %w", err), fmt.Errorf("rollback failed: %w", rollbackErr))
			}
			if rollbackErr := GetManager().RestartProtocol(model.OpenFlux); rollbackErr != nil {
				return nil, errors.Join(fmt.Errorf("restart updated OpenFlux: %w", err), fmt.Errorf("old binary restored but restart failed: %w", rollbackErr))
			}
		}
		if installed {
			return nil, fmt.Errorf("updated OpenFlux failed to start; previous binary restored: %w", err)
		}
		return nil, fmt.Errorf("updated OpenFlux failed to start and was removed: %w", err)
	}
	return GetOpenFluxUpdateInfo(ctx)
}
