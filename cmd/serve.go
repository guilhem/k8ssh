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
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"

	sshserver "github.com/guilhem/k8ssh/pkg/sshServer"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE:         serve,
	SilenceUsage: true,
}

var addr string

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	serveCmd.Flags().StringVarP(&addr, "address", "a", ":2222", "Address to listen")
}

func serve(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	s, err := sshserver.New(addr, config)
	if err != nil {
		return err
	}

	var lc net.ListenConfig

	l, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	if err := s.Serve(l); err != nil {
		return err
	}

	return nil
}
