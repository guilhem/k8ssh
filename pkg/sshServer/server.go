package sshserver

import (
	"fmt"

	"github.com/gliderlabs/ssh"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	server *ssh.Server
}

func New(addr string, config *rest.Config) (*ssh.Server, error) {

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

	server := ssh.Server{
		Addr:             addr,
		Handler:          SshHandler(cl, config),
		PublicKeyHandler: PublicKeyHandler(cl),
		PasswordHandler:  PasswordHandler(),
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": SftpHandler(cl, config),
		},
		ConnCallback:             rl.ConnCallback(),
		ConnectionFailedCallback: rl.ConnectionFailedCallback(),
	}

	return &server, nil
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
