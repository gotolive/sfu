package ice

// Dial creates a new ICE transport.
//	func Dial(candidates []string, ufrag, password string) (Transport, error) {
//		transport := iceTransport{
//			userFragment: ufrag,
//			password:     password,
//			onData:       nil,
//			onState:      nil,
//			role:         RoleControlling,
//		}
//		// for _, c := range candidates {
//		// transport.addRemoteCandidate()
//		// }
//		transport.connect()
//
//		return &transport, nil
//	}
