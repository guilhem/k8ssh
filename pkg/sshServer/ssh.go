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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func SshHandler(cl client.Client, config *rest.Config) ssh.Handler {
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

		// impClientset, err := kubernetes.NewForConfig(impConfig)
		// if err != nil {
		// 	s.Write([]byte(err.Error()))

		// 	return
		// }

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

		cmd, err := command(s.Command(), pod, sa)
		if err != nil {
			log.Printf("Can't get command: %v", err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		_, cWindows, hasPTY := s.Pty()
		queue := sizeQueue{C: cWindows}

		impConfig := rest.CopyConfig(config)

		impConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountName(user.User, user.Namespace),
		}

		restClient, err := rest.RESTClientFor(impConfig)
		if err != nil {
			s.Write([]byte(err.Error()))

			return
		}

		// restClient := impClientset.RESTClient()

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
			TTY:     hasPTY,
		}, scheme.ParameterCodec)

		log.Printf("exec generated URL: %s", req.URL())

		exec, err := remotecommand.NewSPDYExecutor(impConfig, "POST", req.URL())
		if err != nil {
			log.Printf("fail create NewSPDYExecutor for url '%s': %v", req.URL(), err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}

		// type fakeTerminalSizeQueue struct{}

		// var once sync.Once
		// done := make(chan *remotecommand.TerminalSize)

		// func (f *fakefakeTerminalSizeQueue) Next() *remotecommand.TerminalSize {
		// 	once.Do(func() {
		// 		tSize := &remotecommand.TerminalSize{
		// 			Width: uint16(pty.Window.Width),
		// 			Height: uint16(pty.Window.Height),
		// 		}

		// 		done <- tSize
		// 		return
		// 	})

		// 	return <-done
		// }

		// if !ok {
		if err := exec.Stream(remotecommand.StreamOptions{
			Tty:               hasPTY,
			Stdin:             s,
			Stdout:            s,
			Stderr:            s.Stderr(),
			TerminalSizeQueue: queue,
		}); err != nil {
			log.Printf("fail to exec Stream: %v", err)
			s.Stderr().Write([]byte(ErrDestination.Error()))
			s.Exit(1)

			return
		}
	}
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
		return nil, errors.New("can't parse domain")
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
