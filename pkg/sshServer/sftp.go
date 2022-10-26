package sshserver

import (
	"log"

	"github.com/gliderlabs/ssh"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func SftpHandler(clientset *kubernetes.Clientset, config *rest.Config) ssh.SubsystemHandler {
	return func(s ssh.Session) {
		ctx := s.Context()

		user, ok := ctx.Value(User{}).(*User)
		if !ok {
			u, err := getUser(ctx, clientset, ctx.User())
			if err != nil {
				s.Write([]byte(err.Error()))

				return
			}

			ctx.SetValue(User{}, u)

			user = u
		}

		pod, err := clientset.CoreV1().Pods(user.Namespace).Get(ctx, user.Pod, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			log.Printf("Can't find pod %s/%s", user.Namespace, user.Pod)
			s.Exit(1)

			return
		}

		sa, err := clientset.CoreV1().ServiceAccounts(user.Namespace).Get(ctx, user.User, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			log.Printf("Can't find sa %s/%s", user.Namespace, user.User)
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

		exec, err := remotecommand.NewSPDYExecutor(impConfig, "POST", req.URL())
		if err != nil {
			log.Printf("fail create NewSPDYExecutor for url '%s': %v", req.URL(), err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		if err := exec.Stream(remotecommand.StreamOptions{
			Stdin:  s,
			Stdout: s,
			Stderr: s.Stderr(),
		}); err != nil {
			log.Printf("fail to exec Stream: %v", err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		return
	}
}
