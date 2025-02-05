package ice

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/pion/stun"
)

func TestStun(t *testing.T) {
	tests := []testHelper{
		{
			name: "get_username_fail",
			method: func(t *testing.T) {
				m := stun.New()
				l, r := getUsername(m)
				if l != "" || r != "" {
					t.Fail()
				}
			},
		},
		{
			name:        "get_username_success",
			description: "get username from a valid stun message",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				username, r := getUsername(m)
				// username:remoteUsername
				if !strings.HasPrefix(username, stunUsername) || r == "" {
					t.FailNow()
				}
			},
		},
		{
			name: "validateRequestStun",
			method: func(t *testing.T) {
				m := stun.New()
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name:        "validate_stun_success",
			description: "validate a valid stun message",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				code := validateRequestStun(m)
				if code != 0 {
					t.FailNow()
				}
			},
		},
		{
			name:        "validate_stun_message_without_fingerprint",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				m.SetType(stun.MessageType{
					Method: stun.MethodAllocate,
					Class:  stun.ClassRequest,
				})
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name:        "validate_stun_message_without_username",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				var attrs stun.Attributes
				for i, attr := range m.Attributes {
					if attr.Type == stun.AttrUsername {
						attrs = append(attrs, m.Attributes[:i]...)
						attrs = append(attrs, m.Attributes[i+1:]...)
						break
					}
				}
				m.Attributes = attrs
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name:        "validate_stun_message_with_controlled",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				m.Attributes = append(m.Attributes, stun.RawAttribute{
					Type: stun.AttrICEControlled,
				})
				code := validateRequestStun(m)
				assert(t, code, stun.CodeRoleConflict)
			},
		},
		{
			name:        "validate_stun_message_without_fingerprint",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				var attrs stun.Attributes
				for i, attr := range m.Attributes {
					if attr.Type == stun.AttrFingerprint {
						attrs = append(attrs, m.Attributes[:i]...)
						attrs = append(attrs, m.Attributes[i+1:]...)
						break
					}
				}
				m.Attributes = attrs
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name:        "validate_stun_message_without_integrity",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				var attrs stun.Attributes
				for i, attr := range m.Attributes {
					if attr.Type == stun.AttrMessageIntegrity {
						attrs = append(attrs, m.Attributes[:i]...)
						attrs = append(attrs, m.Attributes[i+1:]...)
						break
					}
				}
				m.Attributes = attrs
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name:        "validate_stun_message_without_priority",
			description: "check stun message required attributes",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				var attrs stun.Attributes
				for i, attr := range m.Attributes {
					if attr.Type == stun.AttrPriority {
						attrs = append(attrs, m.Attributes[:i]...)
						attrs = append(attrs, m.Attributes[i+1:]...)
						break
					}
				}
				m.Attributes = attrs
				code := validateRequestStun(m)
				assert(t, code, stun.CodeBadRequest)
			},
		},
		{
			name: "checkAuthenticationSuccess",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				code := checkAuthentication(m, stunUsername, stunPwd)
				if code != 0 {
					t.FailNow()
				}
			},
		},
		{
			name: "checkAuthenticationFail",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				code := checkAuthentication(m, stunUsername+"invalid", stunPwd)
				if code != stun.CodeUnauthorized {
					t.FailNow()
				}
			},
		},
		{
			name: "checkAuthenticationFailPwd",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				code := checkAuthentication(m, stunUsername, stunPwd+"invalid")
				if code != stun.CodeUnauthorized {
					t.FailNow()
				}
			},
		},
		{
			name: "check_authentication_fail",
			method: func(t *testing.T) {
				m := stun.New()
				errCode := checkAuthentication(m, "", "")
				if errCode != stun.CodeBadRequest {
					t.FailNow()
				}
			},
		},
		{
			name: "createErrorResponse",
			method: func(t *testing.T) {
				m := stun.New()
				_, err := createErrorResponse(m, stun.CodeBadRequest)
				if err != nil {
					t.FailNow()
				}
			},
		},
		{
			name: "createErrorResponseSuccess",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				response, err := createErrorResponse(m, stun.CodeBadRequest)
				if err != nil || response == nil {
					t.FailNow()
				}
			},
		},
		{
			name: "createErrorResponseFail",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				_, err := createErrorResponse(m, 200)
				if !errors.Is(err, stun.ErrNoDefaultReason) {
					t.FailNow()
				}
			},
		},
		{
			name: "createBindSuccessResponse",
			method: func(t *testing.T) {
				m := stun.New()
				_, err := createBindSuccessResponse(m, "", nil, "")
				if !errors.Is(err, ErrUnknownProtocol) {
					t.FailNow()
				}
			},
		},
		{
			name: "createUDPBindSuccessResponseSuccess",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				response, err := createBindSuccessResponse(m, UDP, &net.UDPAddr{
					IP:   net.IPv4(0, 0, 0, 0),
					Port: 0,
				}, stunPwd)
				if err != nil || response == nil {
					t.FailNow()
				}
			},
		},
		{
			name: "createTCPBindSuccessResponseSuccess",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				response, err := createBindSuccessResponse(m, TCP, &net.TCPAddr{
					IP:   net.IPv4(0, 0, 0, 0),
					Port: 0,
				}, stunPwd)
				if err != nil || response == nil {
					t.FailNow()
				}
			},
		},
		{
			name: "createTCPBindSuccessResponseFail",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				_, err := createBindSuccessResponse(m, TCP, &net.UDPAddr{
					IP:   net.IPv4(0, 0, 0, 0),
					Port: 0,
				}, stunPwd)
				if !errors.Is(err, ErrNoTCPAddr) {
					t.FailNow()
				}
			},
		},
		{
			name: "createUDPBindSuccessResponseFail",
			method: func(t *testing.T) {
				m := stunMessage(stunBinding)
				_, err := createBindSuccessResponse(m, UDP, &net.TCPAddr{
					IP:   net.IPv4(0, 0, 0, 0),
					Port: 0,
				}, stunPwd)
				if !errors.Is(err, ErrNoUDPAddr) {
					t.FailNow()
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
