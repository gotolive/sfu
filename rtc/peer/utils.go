package peer

import "math/rand"

func RandomString(size int) string {
	dic := []byte{
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b',
		'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n',
		'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	}
	res := make([]byte, size)
	for i := 0; i < size; i++ {
		res[i] = dic[rand.Int()%len(dic)]
	}
	return string(res)
}
