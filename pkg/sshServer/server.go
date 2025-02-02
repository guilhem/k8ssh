package sshserver

import (
	"fmt"
	"log/slog"

	gssh "github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	Server *gssh.Server

	Scheme *runtime.Scheme
	Client client.Client
	Config *rest.Config

	Logger *slog.Logger
}

func New(addr string, config *rest.Config, l *slog.Logger, hostKey any) (*Server, error) {

	// Create a new runtime client
	scheme := runtime.NewScheme()

	// Add scheme for pods and service accounts
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add scheme: %w", err)
	}

	// Create a new runtime client
	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	rl := NewRateLimiter()

	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create host signer: %w", err)
	}

	k8sshServer := &Server{
		Scheme: scheme,
		Client: cl,
		Config: config,
		Logger: l,
	}

	k8sshServer.Server = &gssh.Server{
		Addr:             addr,
		Handler:          k8sshServer.SshHandler(),
		PublicKeyHandler: k8sshServer.PublicKeyHandler(),
		// PasswordHandler:  k8sshServer.PasswordHandler(),
		SubsystemHandlers: map[string]gssh.SubsystemHandler{
			"sftp": k8sshServer.SftpHandler(),
		},
		ConnCallback:             rl.ConnCallback(),
		ConnectionFailedCallback: rl.ConnectionFailedCallback(),
		HostSigners:              []gssh.Signer{hostSigner},
	}

	return k8sshServer, nil
}
