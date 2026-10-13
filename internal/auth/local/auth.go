package local

import (
	"context"
	"net"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/transport"
)

type peerKey int

const (
	peerCredKey peerKey = iota
)

type Authenticator struct {
	Authorized   *grants.Store
	MaxBodyBytes int64
}

type peerInfo struct {
	Cred *transport.PeerCred
	Err  error
}

func WithPeerCred(ctx context.Context, cred *transport.PeerCred, err error) context.Context {
	return context.WithValue(ctx, peerCredKey, peerInfo{Cred: cred, Err: err})
}

func PeerCredFrom(ctx context.Context) (*transport.PeerCred, error) {
	info, _ := ctx.Value(peerCredKey).(peerInfo)
	return info.Cred, info.Err
}

func PrincipalFrom(ctx context.Context) (*principal.Principal, bool) {
	return principal.FromContext(ctx)
}

func WithPrincipal(ctx context.Context, p *principal.Principal) context.Context {
	return principal.WithContext(ctx, p)
}

// Peer credentials come from the kernel, never HTTP headers.
func ConnContext(ctx context.Context, conn net.Conn) context.Context {
	cred, err := transport.PeerCredFromConn(conn)
	return WithPeerCred(ctx, cred, err)
}
