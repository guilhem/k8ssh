package sshserver

import (
	"log"

	"github.com/gliderlabs/ssh"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s Server) SftpHandler() ssh.SubsystemHandler {
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

		cmd, err := command([]string{"/usr/lib/sftp-server", "-e"}, pod, sa)
		if err != nil {
			log.Printf("Can't get command: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}

		impConfig := rest.CopyConfig(s.Config)

		impConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountName(user.User, user.Namespace),
		}

		exec, err := s.RemotecommandExec(impConfig, user.Pod, user.Namespace, cmd, false)
		if err != nil {
			log.Printf("can't create exec: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))

			return
		}

		if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  sshSession,
			Stdout: sshSession,
			Stderr: sshSession.Stderr(),
		}); err != nil {
			log.Printf("fail to exec Stream: %v", err)
			sshSession.Stderr().Write([]byte(ErrDestination.Error()))
			sshSession.Exit(1)

			return
		}
	}
}
