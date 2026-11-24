package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

//go:embed install.sh.tmpl install.ps1.tmpl
var installers embed.FS

func main() {
	version := flag.String("version", "", "agent release version")
	dir := flag.String("dir", "", "directory containing release files")
	format := flag.String("format", "archive", "asset format: archive or binary (binary also emits installation scripts)")
	publicKey := flag.String("public-key", "", "expected signing public key; required for binary releases")
	keyPath := flag.String("private-key", "", "file containing a base64 Ed25519 private key")
	keygen := flag.Bool("generate-key", false, "create the private key file and print its public key")
	flag.Parse()
	if err := run(*version, *dir, *keyPath, *keygen, *format, *publicKey); err != nil {
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

func run(version, dir, keyPath string, generate bool, format, publicKey string) error {
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
	if format != "archive" && format != "binary" {
		return fmt.Errorf("invalid release format")
	}
	encoded, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid Ed25519 private key")
	}
	if !bytes.Equal(ed25519.NewKeyFromSeed(key[:ed25519.SeedSize]), key) {
		return fmt.Errorf("inconsistent Ed25519 private key")
	}
	if format == "binary" && base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Public().(ed25519.PublicKey)) != strings.TrimSpace(publicKey) {
		return fmt.Errorf("signing key does not match the public key embedded in agents")
	}
	manifest := agentupgrade.Manifest{Version: version}
	for _, osName := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := agentupgrade.AssetName(osName, arch, version)
			if format == "binary" {
				name = agentupgrade.BinaryName(osName, arch, version)
			}
			f, err := os.Open(filepath.Join(dir, name))
			if os.IsNotExist(err) && format == "archive" {
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
				return fmt.Errorf("invalid release asset size for %s", name)
			}
			manifest.Assets = append(manifest.Assets, agentupgrade.Asset{OS: osName, Arch: arch, Filename: name, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		}
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("no release assets found")
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(key), data)
	path := filepath.Join(dir, "zadig-agent-v"+version+".manifest.json")
	// Write the signature first; neither existing manifest nor signature may be
	// overwritten. Publish the complete release only after this succeeds.
	if err := create(path+".sig", signature, 0644); err != nil {
		return err
	}
	if err := create(path, data, 0644); err != nil {
		os.Remove(path + ".sig")
		return err
	}
	if format == "binary" {
		for _, file := range []string{"install.sh", "install.ps1"} {
			source, err := installers.ReadFile(file + ".tmpl")
			if err != nil {
				return err
			}
			t, err := template.New(file).Parse(string(source))
			if err != nil {
				return err
			}
			var output bytes.Buffer
			if err := t.Execute(&output, manifest); err != nil {
				return err
			}
			name := strings.Replace(file, "install.", "install-v"+version+".", 1)
			if err := create(filepath.Join(dir, name), output.Bytes(), 0644); err != nil {
				return err
			}
		}
	}
	fmt.Println(path)
	return nil
}
