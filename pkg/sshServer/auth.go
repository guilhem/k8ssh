package sshserver

import (
	"log"

	"github.com/gliderlabs/ssh"
)

func (s Server) PublicKeyHandler() ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		// Get or set user
		user, ok := ctx.Value(User{}).(*User)
		if !ok {
			u, err := getUser(ctx, s.Client, ctx.User())
			if err != nil {
				log.Printf("can't get user: %v", err)

				return false
			}

			ctx.SetValue(User{}, u)

			user = u
		}

		if !ssh.KeysEqual(key, user.PublicKey) {
			log.Printf("PublicKey don't match")

			return false
		}

		return true
	}
}

func PasswordHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, password string) bool {
		log.Print("We don't want password")

		return false
	}
}
