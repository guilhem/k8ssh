package cmd

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestParsePrivateKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	openssh, err := ssh.MarshalPrivateKey(private, "test")
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"OpenSSH Ed25519", pem.EncodeToMemory(openssh), false},
		{"PKCS8 Ed25519", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), false},
		{"invalid", []byte("not a key"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := ParsePrivateKey(tc.data)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParsePrivateKey() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && !public.Equal(signer.Public()) {
				t.Fatal("host public key changed")
			}
		})
	}
}
