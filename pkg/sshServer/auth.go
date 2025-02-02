package sshserver

import (
	"github.com/gliderlabs/ssh"
)

func (s Server) PublicKeyHandler() ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		l := s.Logger.With("user", ctx.User())
		l.Debug("Public key authentication attempt")

		// Get or set user
		user, ok := ctx.Value(User{}).(*User)
		if !ok {
			u, err := getUser(ctx, s.Client, ctx.User())
			if err != nil {
				l.Error("Can't find user", "error", err)

				return false
			}

			ctx.SetValue(User{}, u)

			user = u
		}

		if !ssh.KeysEqual(key, user.PublicKey) {
			l.Error("Invalid public key")
			l.Debug("Public keys", "given_key", string(key.Marshal()), "authorized_key", string(user.PublicKey.Marshal()))

			return false
		}

		return true
	}
}

func (s Server) PasswordHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, password string) bool {
		s.Logger.Debug("Password authentication attempt", "user", ctx.User())

		return false
	}
}
