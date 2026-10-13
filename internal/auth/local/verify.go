package local

import (
	"net/http"
	"strconv"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (a *Authenticator) Authenticate(r *http.Request, _ []byte) (*principal.Principal, *protocol.Error) {
	cred, err := PeerCredFrom(r.Context())
	if err != nil || cred == nil {
		return nil, protocol.NewError(protocol.CodePeerNotAuthorized, "local transport did not report peer credentials")
	}
	u, err := a.Authorized.Lookup(cred.UID)
	if err != nil {
		return nil, protocol.NewError(protocol.CodePeerNotAuthorized, "uid %d is not authorized; use hdkt auth add <username>", cred.UID)
	}
	return principal.New(principal.KindUID, strconv.FormatUint(uint64(cred.UID), 10), u.Role).WithUID(cred.UID).WithGID(cred.GID).WithPID(cred.PID).WithLabel(u.Name), nil
}
