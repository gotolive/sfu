package peer

import (
	"errors"
)

var (
	ErrInvalidPacket    = errors.New("invalid rtpPacket")
	ErrInvalidRtxPacket = errors.New("invalid rtx rtpPacket")
	ErrNoNack           = errors.New("nack not support")
	ErrBadSeq           = errors.New("bad sequence number")
	ErrInvalidSimulcast = errors.New("invalid simulcast params")

	ErrReceiverExist      = errors.New("receiver already exist")
	ErrReceiverNotExist   = errors.New("receiver not exist")
	ErrPayloadNotMatch    = errors.New("payload type not match")
	ErrCodecNotMatch      = errors.New("stream and codec payload type not match")
	ErrRTXPayloadNotMatch = errors.New("rtx payload type not match")
	ErrHeaderIDNotMatch   = errors.New("header id not match in one connection")
	ErrCodecCantBeNil     = errors.New("codec cant be nil")
	ErrStreamCantBeEmpty  = errors.New("streams cant be empty")
	ErrConnExist          = errors.New("connection already exists")
)
