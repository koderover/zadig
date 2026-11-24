package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

func main() {
	version := flag.String("version", "", "agent release version")
	dir := flag.String("dir", "", "directory containing release files")
	format := flag.String("format", "archive", "asset format: archive or binary")
	flag.Parse()
	if err := run(*version, *dir, *format); err != nil {
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

func run(version, dir, format string) error {
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
	manifest := agentupgrade.Manifest{Version: version}
	for _, osName := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if osName == "windows" && arch != "amd64" {
				continue
			}
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
	path := filepath.Join(dir, "zadig-agent-v"+version+".manifest.json")
	if err := create(path, data, 0644); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
