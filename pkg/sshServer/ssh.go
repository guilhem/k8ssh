package sshserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gliderlabs/ssh"
	"github.com/google/shlex"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type User struct {
	PublicKey ssh.PublicKey
	User      string
	Pod       string
	Namespace string
}

var ErrDestination = errors.New("can't find destination")

const AuthorizedKeyAnnotation = "ssh.barpilot.io/publickey"
const CommandAnnotation = "ssh.barpilot.io/command"
const PrefixCommandAnnotation = "ssh.barpilot.io/prefix-command"

func (s Server) SshHandler() ssh.Handler {
	return func(sshSession ssh.Session) {
		ctx := sshSession.Context()

		user, ok := ctx.Value(User{}).(*User)
		if !ok {
			u, err := getUser(ctx, s.Client, ctx.User())
			if err != nil {
				sshSession.Stderr().Write([]byte(err.Error()))

				return
			}

			ctx.SetValue(User{}, u)

			user = u
		}

		pod := &v1.Pod{}
		if err := s.Client.Get(ctx, client.ObjectKey{Namespace: user.Namespace, Name: user.Pod}, pod); err != nil {
			log.Printf("Can't find pod %s/%s: %v", user.Namespace, user.Pod, err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}

		sa := &v1.ServiceAccount{}
		if err := s.Client.Get(ctx, client.ObjectKey{Namespace: user.Namespace, Name: user.User}, sa); err != nil {
			log.Printf("Can't find service account %s/%s: %v", user.Namespace, user.User, err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}

		cmd, err := command(sshSession.Command(), pod, sa)
		if err != nil {
			log.Printf("Can't get command: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}

		_, cWindows, hasPTY := sshSession.Pty()
		queue := sizeQueue{C: cWindows}

		impConfig := rest.CopyConfig(s.Config)

		impConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountName(user.User, user.Namespace),
		}

		exec, err := s.RemotecommandExec(impConfig, user.Pod, user.Namespace, cmd, hasPTY)
		if err != nil {
			log.Printf("can't create exec: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))

			return
		}

		// if !ok {
		if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Tty:               hasPTY,
			Stdin:             sshSession,
			Stdout:            sshSession,
			Stderr:            sshSession.Stderr(),
			TerminalSizeQueue: queue,
		}); err != nil {
			log.Printf("fail to exec Stream: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}
	}
}

func (s Server) RemotecommandExec(config *rest.Config, pod, namespace string, cmd []string, pty bool) (remotecommand.Executor, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("can't get HTTP client: %w", err)
	}

	restClient, err := apiutil.RESTClientForGVK(gvk, false, config, scheme.Codecs, httpClient)
	if err != nil {
		return nil, fmt.Errorf("can't get REST client: %w", err)
	}

	// restClient := impClientset.RESTClient()

	req := restClient.Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     pty,
	}, runtime.NewParameterCodec(scheme.Scheme))

	spdyExec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("can't create SPDY executor: %w", err)
	}

	// WebSocketExecutor must be "GET" method as described in RFC 6455 Sec. 4.1 (page 17).
	websocketExec, err := remotecommand.NewWebSocketExecutor(config, "GET", req.URL().String())
	if err != nil {
		return nil, fmt.Errorf("can't create WebSocket executor: %w", err)
	}

	exec, err := remotecommand.NewFallbackExecutor(websocketExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return nil, fmt.Errorf("can't create fallback executor: %w", err)
	}

	return exec, nil
}

type sizeQueue struct {
	C <-chan ssh.Window
}

func (s sizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.C
	if !ok {
		return nil
	}

	tSize := &remotecommand.TerminalSize{
		Width:  uint16(size.Width),
		Height: uint16(size.Height),
	}

	return tSize
}

func getUser(ctx context.Context, cl client.Client, sshUser string) (*User, error) {
	var u User

	user, domain, ok := strings.Cut(sshUser, "@")
	if !ok {
		return nil, errors.New("can't parse ssh user")
	}

	u.User = user

	pod, namespace, ok := strings.Cut(domain, ".")
	if !ok {
		// If no namespace is provided, use the default namespace
		namespace = "default"
	}

	u.Pod = pod
	u.Namespace = namespace

	sa := &v1.ServiceAccount{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: user}, sa); err != nil {
		return nil, fmt.Errorf("can't get service account: %w", err)
	}

	ann := sa.GetAnnotations()

	authorizedKey, ok := ann[AuthorizedKeyAnnotation]
	if ok {
		sshKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
		if err != nil {
			return nil, err
		}

		u.PublicKey = sshKey
	}

	return &u, nil
}

func command(inputCmd []string, pod *v1.Pod, sa *v1.ServiceAccount) ([]string, error) {
	var cmd []string

	cmdf := func(annotationKey string) ([]string, error) {
		// Get Command from pod annotation
		podAnnoCmd, ok := pod.GetAnnotations()[annotationKey]
		if ok {
			annoCmds, err := shlex.Split(podAnnoCmd)
			if err != nil {
				return nil, fmt.Errorf("can't split command annotation '%s': %w", podAnnoCmd, err)
			}

			return annoCmds, nil
		}

		// Get command from service account annotation
		saAnnoCmd, ok := sa.GetAnnotations()[annotationKey]
		if ok {
			annoCmds, err := shlex.Split(saAnnoCmd)
			if err != nil {
				return nil, fmt.Errorf("can't split command annotation '%s': %w", saAnnoCmd, err)
			}

			return annoCmds, nil
		}

		return nil, nil
	}

	cmd = append(cmd, inputCmd...)
	if len(cmd) == 0 {
		annoCmd, err := cmdf(CommandAnnotation)
		if err != nil {
			return nil, fmt.Errorf("can't get command: %w", err)
		}

		cmd = annoCmd
	}

	prefixCmd, err := cmdf(PrefixCommandAnnotation)
	if err != nil {
		return nil, fmt.Errorf("can't get prefix: %w", err)
	}

	log.Printf("prefixCmd: %+v", prefixCmd)

	return append(prefixCmd, cmd...), nil
}

func serviceAccountName(name, namespace string) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name)
}
