/*
Copyright © 2022 Guilhem Lettron <guilhem@barpilot.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	sshserver "github.com/guilhem/k8ssh/pkg/sshServer"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:          "serve",
	Short:        "Start the ssh server",
	RunE:         serve,
	SilenceUsage: true,
}

var addr string
var keyPath string

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&addr, "address", "a", ":2222", "Address to listen")

	serveCmd.Flags().StringVarP(&keyPath, "hostkey", "k", "", "Path to the host private key")
}

func serve(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cliconfig := genericclioptions.NewConfigFlags(false)
	restConfig, err := cliconfig.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get rest config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var key crypto.Signer

	if keyPath == "" {

		genkey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}
		key = genkey
	} else {
		// Read the private key
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}

		parseKey, err := ParsePrivateKey(keyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}

		key = parseKey
	}

	s, err := sshserver.New(addr, restConfig, logger, key)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	var lc net.ListenConfig

	listerner, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	if err := s.Server.Serve(listerner); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func ParsePrivateKey(pemBytes []byte) (crypto.Signer, error) {
	// Decode the PEM block
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("can't decode PEM block")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return k, nil
		case *ecdsa.PrivateKey:
			return k, nil
		default:
			return nil, errors.New("key type not supported")
		}
	}

	return nil, errors.New("key type not supported")
}
