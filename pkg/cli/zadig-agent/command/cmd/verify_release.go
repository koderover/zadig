package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"github.com/spf13/cobra"
)

func init() {
	verify := &cobra.Command{Use: "verify-release", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("manifest")
		read := func(path string, limit int64) ([]byte, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, limit+1))
			if int64(len(data)) > limit {
				return nil, fmt.Errorf("release metadata exceeds size limit")
			}
			return data, err
		}
		data, err := read(path, 1<<20)
		if err != nil {
			return err
		}
		signature, err := read(path+".sig", 64)
		if err != nil {
			return err
		}
		key, err := agentupgrade.PublicKey(config.BuildUpgradePublicKey)
		if err != nil {
			return err
		}
		manifest, err := agentupgrade.VerifyManifest(data, signature, config.BuildAgentVersion, key)
		if err != nil {
			return err
		}
		asset, err := manifest.Asset(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return err
		}
		if asset.Filename != agentupgrade.BinaryName(runtime.GOOS, runtime.GOARCH, manifest.Version) {
			return fmt.Errorf("installer requires a binary release")
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		f, err := os.Open(executable)
		if err != nil {
			return err
		}
		defer f.Close()
		hash := sha256.New()
		size, err := io.Copy(hash, io.LimitReader(f, asset.Size+1))
		if err != nil {
			return err
		}
		if size != asset.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), asset.SHA256) {
			return fmt.Errorf("agent binary does not match signed release")
		}
		wait, _ := cmd.Flags().GetBool("wait-for-stop")
		if wait {
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
			defer cancel()
			lease, err := updater.WaitForInstance(ctx)
			if err != nil {
				return fmt.Errorf("old agent has not stopped: %w", err)
			}
			defer lease.Close()
		}
		return nil
	}}
	verify.Flags().String("manifest", "", "Signed release manifest path")
	verify.Flags().Bool("wait-for-stop", false, "Wait for the existing agent to release its instance lock")
	rootCmd.AddCommand(verify)
}
