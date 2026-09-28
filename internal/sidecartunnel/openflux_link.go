package sidecartunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

var ErrOpenFluxLinkToolUnavailable = errors.New("OpenFlux canonical link tool is unavailable")

// MakeOpenFluxLink asks the installed OpenFlux core to normalize and encode a
// share configuration. Passing the secret over stdin keeps it out of the
// process list. This deliberately makes link generation follow the upgraded
// core instead of maintaining a second copy of its rules in the panel.
func MakeOpenFluxLink(ctx context.Context, configJSON []byte) (string, error) {
	bin, err := binaryPath(model.OpenFlux)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOpenFluxLinkToolUnavailable, err)
	}
	cmd := exec.CommandContext(ctx, bin, "--make-link", "-")
	cmd.Stdin = bytes.NewReader(configJSON)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("OpenFlux --make-link: %w: %s", err, detail)
		}
		return "", fmt.Errorf("OpenFlux --make-link: %w", err)
	}
	var result struct {
		Link  string `json:"link"`
		Error string `json:"error"`
		Code  string `json:"code"`
		Param string `json:"param"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		return "", fmt.Errorf("OpenFlux --make-link returned invalid JSON: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("OpenFlux link rejected (%s%s): %s", result.Code, result.Param, result.Error)
	}
	if !strings.HasPrefix(result.Link, "openflux://v1/") {
		return "", errors.New("OpenFlux --make-link returned no link")
	}
	return result.Link, nil
}
