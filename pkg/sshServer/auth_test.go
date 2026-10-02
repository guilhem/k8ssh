package sshserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetUserRequiresAuthorizedKey(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, annotation string
		wantErr          bool
	}{
		{"missing", "", true},
		{"invalid", "not a public key", true},
		{"valid", string(ssh.MarshalAuthorizedKey(key)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := v1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			account := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
				Name: "alice", Namespace: "default",
			}}
			if tc.annotation != "" {
				account.Annotations = map[string]string{AuthorizedKeyAnnotation: tc.annotation}
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(account).Build()
			user, err := getUser(context.Background(), cl, "alice@pod.default")
			if (err != nil) != tc.wantErr {
				t.Fatalf("getUser() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && (user.PublicKey == nil || string(user.PublicKey.Marshal()) != string(key.Marshal())) {
				t.Fatal("authorized key was not retained")
			}
		})
	}
}
