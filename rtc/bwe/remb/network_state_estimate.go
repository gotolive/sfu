package remb

// Not implements.
type NetworkStateEstimate struct {
	// link_capacity
}

func (e *NetworkStateEstimate) LinkCapacityLower() Bitrate {
	panic("implement me")
}
