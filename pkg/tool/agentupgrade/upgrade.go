package agentupgrade

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

type Status string

const (
	Pending      Status = "pending"
	Downloading  Status = "downloading"
	Draining     Status = "draining"
	Restarting   Status = "restarting"
	Verifying    Status = "verifying"
	Succeeded    Status = "succeeded"
	Failed       Status = "failed"
	RolledBack   Status = "rolled_back"
	MaxAssetSize int64  = 512 << 20
)

type Task struct {
	Force         bool   `bson:"force" json:"force"`
	ID            string `bson:"id" json:"id"`
	FromVersion   string `bson:"from_version" json:"from_version"`
	TargetVersion string `bson:"target_version" json:"target_version"`
	ManifestURL   string `bson:"manifest_url" json:"-"`
	Status        Status `bson:"status" json:"status"`
	Error         string `bson:"error" json:"error"`
	RequestedBy   string `bson:"requested_by" json:"requested_by"`
	CreateTime    int64  `bson:"create_time" json:"create_time"`
	UpdateTime    int64  `bson:"update_time" json:"update_time"`
	FinishTime    int64  `bson:"finish_time" json:"finish_time"`
}

type Report struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Instruction struct {
	Force         bool   `json:"force"`
	ID            string `json:"id"`
	TargetVersion string `json:"target_version"`
	ManifestURL   string `json:"manifest_url"`
}

type Manifest struct {
	Version string  `json:"version"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

func (s Status) Terminal() bool { return s == Succeeded || s == Failed || s == RolledBack }

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}
	if from.Terminal() {
		return false
	}
	if to == Succeeded {
		return from == Verifying
	}
	if to == Failed {
		return true
	}
	if to == RolledBack {
		return true
	}
	order := map[Status]int{Pending: 0, Downloading: 1, Draining: 2, Restarting: 3, Verifying: 4, Succeeded: 5}
	a, okA := order[from]
	b, okB := order[to]
	return okA && okB && b > a
}

var versionPattern = regexp.MustCompile(`^[0-9][0-9A-Za-z.+-]{0,63}$`)
var digits = regexp.MustCompile(`^[0-9]+$`)

func Version(v string) (string, error) {
	v = strings.TrimPrefix(v, "v")
	if !versionPattern.MatchString(v) {
		return "", fmt.Errorf("invalid agent version")
	}
	return v, nil
}

func Newer(target, current string) bool {
	target, err := Version(target)
	if err != nil {
		return false
	}
	current, err = Version(current)
	if err != nil {
		return false
	}
	if digits.MatchString(target) || digits.MatchString(current) {
		if !digits.MatchString(target) || !digits.MatchString(current) {
			return false
		}
		a, _ := new(big.Int).SetString(target, 10)
		b, _ := new(big.Int).SetString(current, 10)
		return a.Cmp(b) > 0
	}
	a, err := semver.ParseTolerant(target)
	if err != nil {
		return false
	}
	b, err := semver.ParseTolerant(current)
	return err == nil && a.GT(b)
}

func ManifestURL(base, version string) (string, error) {
	v, err := Version(version)
	if err != nil {
		return "", err
	}
	u, err := SecureURL(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/zadig-agent-v" + v + ".manifest.json"
	return u.String(), nil
}

func SecureURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("agent upgrade requires an HTTPS URL without credentials, query or fragment")
	}
	return u, nil
}

func AssetName(osName, arch, version string) string {
	return fmt.Sprintf("zadig-agent-%s-%s-v%s.tar.gz", osName, arch, version)
}

func BinaryName(osName, arch, version string) string {
	name := fmt.Sprintf("zadig-agent-%s-%s-v%s", osName, arch, version)
	if osName == "windows" {
		name += ".exe"
	}
	return name
}

func (m *Manifest) Asset(osName, arch string) (*Asset, error) {
	if osName == "windows" && arch != "amd64" {
		return nil, fmt.Errorf("unsupported upgrade platform %s/%s", osName, arch)
	}
	var result *Asset
	for i := range m.Assets {
		a := &m.Assets[i]
		if a.OS == osName && a.Arch == arch {
			if result != nil {
				return nil, fmt.Errorf("duplicate upgrade asset")
			}
			result = a
		}
	}
	if result == nil {
		return nil, fmt.Errorf("no upgrade asset for %s/%s", osName, arch)
	}
	digest, err := hex.DecodeString(result.SHA256)
	if err != nil || len(digest) != 32 || result.Size <= 0 || result.Size > MaxAssetSize || (result.Filename != AssetName(osName, arch, m.Version) && result.Filename != BinaryName(osName, arch, m.Version)) {
		return nil, fmt.Errorf("invalid upgrade asset")
	}
	return result, nil
}

func TLSConfig() (*tls.Config, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if path := os.Getenv("ZADIG_AGENT_UPGRADE_CA_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("invalid agent upgrade CA file")
		}
		config.RootCAs = pool
	}
	return config, nil
}

func HTTPClient() (*http.Client, error) {
	tlsConfig, err := TLSConfig()
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{Transport: transport, Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" {
			return fmt.Errorf("unsafe upgrade redirect")
		}
		return nil
	}}, nil
}

func Get(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	if _, err := SecureURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upgrade download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("upgrade download exceeds size limit")
	}
	return data, nil
}

func FetchManifest(ctx context.Context, client *http.Client, raw, version string) (*Manifest, error) {
	data, err := Get(ctx, client, raw, 1<<20)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data, version)
}

func ParseManifest(data []byte, version string) (*Manifest, error) {
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("upgrade manifest exceeds size limit")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != strings.TrimPrefix(version, "v") {
		return nil, fmt.Errorf("upgrade manifest version mismatch")
	}
	return &manifest, nil
}

func AssetURL(manifestURL, filename string) (string, error) {
	u, err := SecureURL(manifestURL)
	if err != nil {
		return "", err
	}
	if path.Base(filename) != filename || strings.ContainsAny(filename, `/\`) {
		return "", fmt.Errorf("invalid asset filename")
	}
	u.Path = path.Join(path.Dir(u.Path), filename)
	return u.String(), nil
}
