package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

func main() {
	version := flag.String("version", "", "agent release version")
	dir := flag.String("dir", "", "directory containing release files")
	signingKey := flag.String("signing-key-file", "", "file containing a base64 Ed25519 private key; never put the key on the command line")
	flag.Parse()
	if err := runSigned(*version, *dir, *signingKey); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func create(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		os.Remove(path)
		return err
	}
	return closeErr
}

func run(version, dir string) error { return runSigned(version, dir, "") }

func runSigned(version, dir, signingKeyFile string) error {
	var key ed25519.PrivateKey
	if signingKeyFile != "" {
		data, err := os.ReadFile(signingKeyFile)
		if err != nil {
			return err
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil || len(decoded) != ed25519.PrivateKeySize {
			return fmt.Errorf("invalid release signing private key")
		}
		key = ed25519.PrivateKey(decoded)
	}
	if version == "" {
		return fmt.Errorf("version is required")
	}
	if dir == "" {
		return fmt.Errorf("dir is required")
	}
	manifest := agentupgrade.Manifest{Version: version}
	for _, osName := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if osName == "windows" && arch != "amd64" {
				continue
			}
			name := agentupgrade.BinaryName(osName, arch, version)
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			hash := sha256.New()
			size, err := io.Copy(hash, io.LimitReader(f, agentupgrade.MaxAssetSize+1))
			f.Close()
			if err != nil {
				return err
			}
			if size <= 0 || size > agentupgrade.MaxAssetSize {
				return fmt.Errorf("invalid release asset size for %s", name)
			}
			manifest.Assets = append(manifest.Assets, agentupgrade.Asset{OS: osName, Arch: arch, Filename: name, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "zadig-agent-v"+version+".manifest.json")
	if err := create(path, data, 0644); err != nil {
		return err
	}
	if key != nil {
		if err := create(path+".sig", ed25519.Sign(key, data), 0644); err != nil {
			os.Remove(path)
			return err
		}
	}
	fmt.Println(path)
	return nil
}
