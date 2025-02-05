package ice

import (
	"encoding/binary"
	"net"
	"strings"

	"github.com/pion/stun"
)

// some stun helper method.

const (
	attrPrioritySize = 4
)

func validateBindingStun(m *stun.Message, ufrag, pwd string) stun.ErrorCode {
	if errCode := validateRequestStun(m); errCode != 0 {
		return errCode
	}
	return checkAuthentication(m, ufrag, pwd)
}

func getUsername(m *stun.Message) (string, string) { //nolint:unparam
	username, err := m.Get(stun.AttrUsername)
	if err != nil {
		return "", ""
	}
	ufrags := strings.Split(string(username), ":")
	if len(ufrags) != 2 {
		return "", ""
	}
	return ufrags[0], ufrags[1]
}

func validateRequestStun(m *stun.Message) stun.ErrorCode {
	// as a server we only handle request binding with fingerprint atte
	if m.Type.Class != stun.ClassRequest || m.Type.Method != stun.MethodBinding {
		return stun.CodeBadRequest
	}

	// validate the attrs we need.
	if !m.Contains(stun.AttrFingerprint) || !m.Contains(stun.AttrMessageIntegrity) {
		return stun.CodeBadRequest
	}

	// validate priority
	priority, err := m.Get(stun.AttrPriority)
	if err != nil || len(priority) != attrPrioritySize || binary.BigEndian.Uint32(priority) == 0 {
		return stun.CodeBadRequest
	}

	// validate username
	username, err := m.Get(stun.AttrUsername)
	if err != nil || len(username) == 0 {
		return stun.CodeBadRequest
	}

	// validate the ice role
	if m.Contains(stun.AttrICEControlled) {
		return stun.CodeRoleConflict
	}
	return 0
}

func checkAuthentication(m *stun.Message, ufrag, password string) stun.ErrorCode {
	// validate username
	username, err := m.Get(stun.AttrUsername)
	if err != nil || len(username) == 0 {
		return stun.CodeBadRequest
	}
	if len(username) < len(ufrag) || username[len(ufrag)] != ':' || string(username[:len(ufrag)]) != ufrag {
		return stun.CodeUnauthorized
	}
	i := stun.NewShortTermIntegrity(password)
	if err := i.Check(m); err != nil {
		return stun.CodeUnauthorized
	}

	return 0
}

func createErrorResponse(m *stun.Message, code stun.ErrorCode) (*stun.Message, error) {
	message := stun.New()
	message.SetType(stun.MessageType{
		Method: m.Type.Method,
		Class:  stun.ClassErrorResponse,
	})
	message.TransactionID = m.TransactionID
	if err := code.AddTo(message); err != nil {
		return nil, err
	}
	return message, nil
}

func createBindSuccessResponse(m *stun.Message, protocol string, remoteAddr net.Addr, pwd string) (*stun.Message, error) {
	var ip net.IP
	var port int

	switch protocol {
	case TCP:
		if addr, ok := remoteAddr.(*net.TCPAddr); ok {
			ip = addr.IP
			port = int(addr.AddrPort().Port())
		} else {
			return nil, ErrNoTCPAddr
		}
	case UDP:
		if addr, ok := remoteAddr.(*net.UDPAddr); ok {
			ip = addr.IP
			port = addr.Port
		} else {
			return nil, ErrNoUDPAddr
		}
	default:
		return nil, ErrUnknownProtocol
	}

	message, err := stun.Build(m, stun.BindingSuccess,
		&stun.XORMappedAddress{
			IP:   ip,
			Port: port,
		},
		stun.NewShortTermIntegrity(pwd),
		stun.Fingerprint,
	)
	//
	if err != nil {
		return nil, err
	}
	return message, nil
}
