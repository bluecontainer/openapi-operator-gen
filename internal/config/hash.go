package config

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// HashSpecFile computes the SHA-256 hash of a spec file or URL.
// Returns a string in the format "sha256:<hex>".
func HashSpecFile(specPath string) (string, error) {
	var data []byte

	if strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://") {
		resp, err := http.Get(specPath)
		if err != nil {
			return "", fmt.Errorf("failed to download spec: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to download spec: HTTP %d", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read spec content: %w", err)
		}
	} else {
		var err error
		data, err = os.ReadFile(specPath)
		if err != nil {
			return "", fmt.Errorf("failed to read spec file: %w", err)
		}
	}

	return HashSpecBytes(data), nil
}

// HashSpecBytes computes the SHA-256 hash of raw spec content.
// Returns a string in the format "sha256:<hex>".
func HashSpecBytes(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h)
}
