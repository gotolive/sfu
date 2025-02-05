package ice

import (
	"fmt"
	"testing"
)

func Test_getInterfaceIps(t *testing.T) {
	ips, err := getInterfaceIps(true)
	fmt.Println(ips, err)
}
