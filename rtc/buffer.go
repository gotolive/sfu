package rtc

type CowBuffer []byte

func (b CowBuffer) Copy() CowBuffer {
	a := make([]byte, len(b))
	copy(a, b)
	return a
}
