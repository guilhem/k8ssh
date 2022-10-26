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
	"context"
	"errors"

	"github.com/gliderlabs/ssh"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
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
	PreRunE:      preServe,
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

type configKey struct{}

func preServe(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	ctx = context.WithValue(ctx, configKey{}, config)
	cmd.SetContext(ctx)

	return nil
}

func serve(cmd *cobra.Command, args []string) error {
	config, ok := cmd.Context().Value(configKey{}).(*rest.Config)
	if !ok {
		return errors.New("can't get kclient")
	}

	if err := setKubernetesDefaults(config); err != nil {
		return err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// callBack := func(ctx ssh.Context, conn net.Conn) net.Conn {
	// 	return conn
	// }

	server := ssh.Server{
		Addr:             addr,
		Handler:          sshserver.SshHandler(clientset, config),
		PublicKeyHandler: sshserver.PublicKeyHandler(clientset),
		PasswordHandler:  sshserver.PasswordHandler(),
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sshserver.SftpHandler(clientset, config),
		},
	}

	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

// setKubernetesDefaults sets default values on the provided client config for accessing the
// Kubernetes API or returns an error if any of the defaults are impossible or invalid.
// TODO this isn't what we want.  Each clientset should be setting defaults as it sees fit.
func setKubernetesDefaults(config *rest.Config) error {
	// TODO remove this hack.  This is allowing the GetOptions to be serialized.
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}

	if config.APIPath == "" {
		config.APIPath = "/api"
	}

	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}
	return rest.SetKubernetesDefaults(config)
}
