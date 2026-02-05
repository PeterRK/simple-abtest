package utils

type Xorshift struct {
	x, y, z, w uint32
}

func (xs *Xorshift) Init(seed uint32) {
	xs.x, xs.y, xs.z = 0x6c078965, 0x9908b0df, 0x9d2c5680
	xs.w = seed
}

func (xs *Xorshift) Next() uint32 {
	t := xs.x ^ (xs.x << 11)
	xs.x, xs.y, xs.z = xs.y, xs.z, xs.w
	xs.w ^= (xs.w >> 19) ^ t ^ (t >> 8)
	return xs.w
}
