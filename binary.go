package binary

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Binary represents a executable binary.
type Binary struct {
	Name string
	// Path to save the binary to
	DownloadPath string
	// URL to download binary from. May contain environment variables.
	// $GOOS, $GOARCH and $VERSION will be ensured to be set.
	URL string
	// Default value for $VERSION
	DefaultVersion string
	// Base64 encoded version of the binary
	Data string
}

// Path for the binary
func (b Binary) ExecutablePath() string {
	rel := filepath.Join(b.DownloadPath, b.Name)
	abs, err := filepath.Abs(rel)
	if err != nil {
		return rel
	}
	return abs
}

func (b Binary) ExpandURL() string {
	url := b.URL
	m := map[string]string{
		"GOOS":    runtime.GOOS,
		"GOARCH":  runtime.GOARCH,
		"VERSION": b.DefaultVersion,
	}
	url = os.Expand(url, func(key string) string {
		val, ok := m[key]
		if !ok {
			return os.Getenv(key)
		}
		return val
	})
	return url
}

// Downloads the binary from the given URL, encodes it using base64
// and stores it in the data field (i.e. in memory).
func (b *Binary) Download() error {
	resp, err := http.Get(b.ExpandURL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := &bytes.Buffer{}
	enc := base64.NewEncoder(base64.StdEncoding, buf)
	_, err = io.Copy(enc, resp.Body)
	enc.Close()
	b.Data = string(buf.Bytes())
	return err
}

// Decodes the data field and stores it in the executable path.
func (b Binary) Save() error {
	f, err := os.OpenFile(b.ExecutablePath(), os.O_CREATE|os.O_WRONLY, os.FileMode(0755))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(b.Data))))
	return err
}

// Downloads the binary from the given URL and saves it directly
// to the executable path without storing it in memory.
func (b Binary) DownloadAndSave() error {
	resp, err := http.Get(b.ExpandURL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(b.ExecutablePath())
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func (b *Binary) IsSaved() bool {
	info, err := os.Lstat(b.ExecutablePath())
	if err == nil && info.Mode()&os.ModeType == os.FileMode(0) {
		return true
	}
	return false
}

func (b *Binary) Prepare() error {
	alreadyExtracted := b.IsSaved()
	if !alreadyExtracted && b.Data == "" {
		err := b.DownloadAndSave()
		if err != nil {
			return err
		}
	} else if !alreadyExtracted && b.Data != "" {
		err := b.Save()
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Binary) Run(args []string) error {
	cmd := exec.Command(b.ExecutablePath(), args...)

	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (b *Binary) PrepareAndRun(args []string) {
	err := b.Prepare()
	if err != nil {
		log.Fatalf("Could not prepare executable: %s", err)
	}
	err = b.Run(args)
	if err != nil {
		log.Fatalf("Could not run executable: %s", err)
	}
}
