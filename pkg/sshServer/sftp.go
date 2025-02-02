package sshserver

import (
	"log"

	"github.com/gliderlabs/ssh"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SftpHandler(cl client.Client, config *rest.Config) ssh.SubsystemHandler {
	return func(s ssh.Session) {
		ctx := s.Context()

		user, ok := ctx.Value(User{}).(*User)
		if !ok {
			u, err := getUser(ctx, cl, ctx.User())
			if err != nil {
				s.Stderr().Write([]byte(err.Error()))

				return
			}

			ctx.SetValue(User{}, u)

			user = u
		}

		pod := &v1.Pod{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: user.Namespace, Name: user.Pod}, pod); err != nil {
			log.Printf("Can't find pod %s/%s: %v", user.Namespace, user.Pod, err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		sa := &v1.ServiceAccount{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: user.Namespace, Name: user.User}, sa); err != nil {
			log.Printf("Can't find service account %s/%s: %v", user.Namespace, user.User, err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		cmd, err := command([]string{"/usr/lib/sftp-server", "-e"}, pod, sa)
		if err != nil {
			log.Printf("Can't get command: %v", err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		impConfig := rest.CopyConfig(config)

		impConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountName(user.User, user.Namespace),
		}

		restClient, err := rest.RESTClientFor(impConfig)
		if err != nil {
			s.Stderr().Write([]byte(err.Error()))
			s.Exit(1)

			return
		}

		req := restClient.Post().
			Resource("pods").
			Name(user.Pod).
			Namespace(user.Namespace).
			SubResource("exec")

		req.VersionedParams(&v1.PodExecOptions{
			Command: cmd,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

		spdyExec, err := remotecommand.NewSPDYExecutor(impConfig, "POST", req.URL())
		if err != nil {
			log.Printf("fail create NewSPDYExecutor for url '%s': %v", req.URL(), err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		// WebSocketExecutor must be "GET" method as described in RFC 6455 Sec. 4.1 (page 17).
		websocketExec, err := remotecommand.NewWebSocketExecutor(impConfig, "GET", req.URL().String())
		if err != nil {
			return
		}

		exec, err := remotecommand.NewFallbackExecutor(websocketExec, spdyExec, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})
		if err != nil {
			return
		}

		if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  s,
			Stdout: s,
			Stderr: s.Stderr(),
		}); err != nil {
			log.Printf("fail to exec Stream: %v", err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}
	}
}
