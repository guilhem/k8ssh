package sshserver

import (
	"net"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	cache "github.com/go-pkgz/expirable-cache/v2"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	c cache.Cache[string, *rate.Limiter]
}

func NewRateLimiter() *RateLimiter {
	c := cache.NewCache[string, *rate.Limiter]().WithTTL(time.Minute * 10)

	return &RateLimiter{
		c: c,
	}
}

func (r *RateLimiter) ConnCallback() ssh.ConnCallback {
	return func(ctx ssh.Context, conn net.Conn) net.Conn {
		lim, ok := r.c.Get(getIP(conn.RemoteAddr()))
		if ok {
			lim.Wait(ctx)
		}

		return conn
	}
}

func (r *RateLimiter) ConnectionFailedCallback() ssh.ConnectionFailedCallback {
	return func(conn net.Conn, err error) {
		if err != nil {
			ip := getIP(conn.RemoteAddr())

			lim, ok := r.c.Get(ip)
			if !ok {
				lim := rate.NewLimiter(rate.Every(time.Second), 1)
				r.c.Set(ip, lim, 0)

				return
			}

			// increase limit
			lim.SetLimit(lim.Limit() / 2)
		}
	}
}

func getIP(addr net.Addr) string {
	ret, _, _ := strings.Cut(addr.String(), ":")

	return ret
}
