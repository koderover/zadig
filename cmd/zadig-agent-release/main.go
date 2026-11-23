package main

import (
	"crypto/ed25519"
	"crypto/rand"
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
	dir := flag.String("dir", "", "directory containing release archives")
	keyPath := flag.String("private-key", "", "file containing a base64 Ed25519 private key")
	keygen := flag.Bool("generate-key", false, "create the private key file and print its public key")
	flag.Parse()
	if err := run(*version, *dir, *keyPath, *keygen); err != nil {
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

func run(version, dir, keyPath string, generate bool) error {
	if keyPath == "" {
		return fmt.Errorf("private-key is required")
	}
	if generate {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		if err := create(keyPath, []byte(base64.StdEncoding.EncodeToString(private)+"\n"), 0600); err != nil {
			return err
		}
		fmt.Println(base64.StdEncoding.EncodeToString(public))
		return nil
	}
	version, err := agentupgrade.Version(version)
	if err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("dir is required")
	}
	encoded, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid Ed25519 private key")
	}
	manifest := agentupgrade.Manifest{Version: version}
	for _, osName := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := agentupgrade.AssetName(osName, arch, version)
			f, err := os.Open(filepath.Join(dir, name))
			if os.IsNotExist(err) {
				continue
			}
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
				return fmt.Errorf("invalid archive size for %s", name)
			}
			manifest.Assets = append(manifest.Assets, agentupgrade.Asset{OS: osName, Arch: arch, Filename: name, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		}
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("no release archives found")
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(key), data)
	path := filepath.Join(dir, "zadig-agent-v"+version+".manifest.json")
	// Write the signature first; neither existing manifest nor signature may be
	// overwritten. Publish both files and the archives only after this succeeds.
	if err := create(path+".sig", signature, 0644); err != nil {
		return err
	}
	if err := create(path, data, 0644); err != nil {
		os.Remove(path + ".sig")
		return err
	}
	fmt.Println(path)
	return nil
}
