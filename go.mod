module github.com/gotolive/sfu

go 1.23

require (
	github.com/gorilla/websocket v1.5.3
	github.com/pion/dtls/v2 v2.2.12
	github.com/pion/logging v0.2.2
	github.com/pion/rtcp v1.2.10
	github.com/pion/rtp v1.8.1
	github.com/pion/srtp/v2 v2.0.17
	github.com/pion/stun v0.6.1
)

require (
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/transport/v2 v2.2.4 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/pion/rtp v1.8.1 => github.com/jerry-tao/rtp v0.0.0-20230728164556-42b21a5d5532
