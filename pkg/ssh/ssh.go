package ssh

import (
	"fmt"

	"github.com/gliderlabs/ssh"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sshv1alpha1 "github.com/guilhem/k8ssh/api/v1alpha1"
)

func NewServer(client client.Client, config *rest.Config) *ssh.Server {
	server := &ssh.Server{
		Addr: ":2222",
		Handler: func(sess ssh.Session) {
			handler(sess, client, config)
		},
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return passwordHandler(ctx, password, client)
		},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			return publicKeyHandler(ctx, key, client)
		},
	}
	return server
}

func passwordHandler(ctx ssh.Context, password string, client client.Client) bool {
	conn := new(k8sshv1alpha1.Connection)
	if err := client.Get(ctx, types.NamespacedName{Name: ctx.User()}, conn); err != nil {
		return false
	}

	return conn.Spec.Password == password
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey, client client.Client) bool {
	conn := new(k8sshv1alpha1.Connection)
	if err := client.Get(ctx, types.NamespacedName{Name: ctx.User()}, conn); err != nil {
		return false
	}

	for _, authKey := range conn.Spec.AuthorizedKeys {
		k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authKey))
		if err != nil {
			continue
		}
		if ssh.KeysEqual(key, k) {
			return true
		}
	}

	return false
}

func handler(sess ssh.Session, client client.Client, config *rest.Config) {
	conn := new(k8sshv1alpha1.Connection)
	if err := client.Get(sess.Context(), types.NamespacedName{Name: sess.User()}, conn); err != nil {
		fmt.Fprintf(sess, "fail to get connection: %v\n", err)
		return
	}

	cmd := sess.Command()
	if len(cmd) == 0 {
		cmd = conn.Spec.Command
	}

	if err := connectPod(sess, config, types.NamespacedName{Name: conn.Spec.Pod.Name, Namespace: conn.Spec.Pod.Namespace}, cmd); err != nil {
		fmt.Fprintf(sess, "error connectpod: %v\n", err)
	}
}

type sshTerminalSizeQueue struct {
	windows <-chan ssh.Window
}

func (q sshTerminalSizeQueue) Next() *remotecommand.TerminalSize {
	window, ok := <-q.windows
	if !ok {
		// channel is closed
		return nil
	}
	return &remotecommand.TerminalSize{
		Width:  uint16(window.Width),
		Height: uint16(window.Height),
	}
}

func connectPod(sess ssh.Session, config *rest.Config, key client.ObjectKey, cmd []string) error {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("can't get client: %w", err)
	}

	_, resize, isPty := sess.Pty()
	termSize := sshTerminalSizeQueue{windows: resize}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(key.Name).
		Namespace(key.Namespace).
		SubResource("exec").
		VersionedParams(
			&v1.PodExecOptions{
				Command: cmd,
				Stdin:   true,
				Stdout:  true,
				Stderr:  true,
				TTY:     isPty,
			}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:             sess,
		Stdout:            sess,
		Stderr:            sess.Stderr(),
		Tty:               isPty,
		TerminalSizeQueue: termSize,
	})
}
