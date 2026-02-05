package core

import (
	"unsafe"
)

func getUint16(data []byte) uint16 {
	return *(*uint16)(unsafe.Pointer(&data[0]))
}

func getUint32(data []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&data[0]))
}

func getUint64(data []byte) uint64 {
	return *(*uint64)(unsafe.Pointer(&data[0]))
}

func rot(x uint64, k int) uint64 {
	return (x << k) | (x >> (64 - k))
}

type state struct {
	a, b, c, d uint64
}

func (s *state) mix() {
	s.c = rot(s.c, 50)
	s.c += s.d
	s.a ^= s.c
	s.d = rot(s.d, 52)
	s.d += s.a
	s.b ^= s.d
	s.a = rot(s.a, 30)
	s.a += s.b
	s.c ^= s.a
	s.b = rot(s.b, 41)
	s.b += s.c
	s.d ^= s.b
	s.c = rot(s.c, 54)
	s.c += s.d
	s.a ^= s.c
	s.d = rot(s.d, 48)
	s.d += s.a
	s.b ^= s.d
	s.a = rot(s.a, 38)
	s.a += s.b
	s.c ^= s.a
	s.b = rot(s.b, 37)
	s.b += s.c
	s.d ^= s.b
	s.c = rot(s.c, 62)
	s.c += s.d
	s.a ^= s.c
	s.d = rot(s.d, 34)
	s.d += s.a
	s.b ^= s.d
	s.a = rot(s.a, 5)
	s.a += s.b
	s.c ^= s.a
	s.b = rot(s.b, 36)
	s.b += s.c
	s.d ^= s.b
}

func (s *state) end() {
	s.d ^= s.c
	s.c = rot(s.c, 15)
	s.d += s.c
	s.a ^= s.d
	s.d = rot(s.d, 52)
	s.a += s.d
	s.b ^= s.a
	s.a = rot(s.a, 26)
	s.b += s.a
	s.c ^= s.b
	s.b = rot(s.b, 51)
	s.c += s.b
	s.d ^= s.c
	s.c = rot(s.c, 28)
	s.d += s.c
	s.a ^= s.d
	s.d = rot(s.d, 9)
	s.a += s.d
	s.b ^= s.a
	s.a = rot(s.a, 47)
	s.b += s.a
	s.c ^= s.b
	s.b = rot(s.b, 54)
	s.c += s.b
	s.d ^= s.c
	s.c = rot(s.c, 32)
	s.d += s.c
	s.a ^= s.d
	s.d = rot(s.d, 25)
	s.a += s.d
	s.b ^= s.a
	s.a = rot(s.a, 63)
	s.b += s.a
}

func Hash128(seed uint64, data []byte) [2]uint64 {
	const magic uint64 = 0xdeadbeefdeadbeef
	s := state{seed, seed, magic, magic}
	l := uint64(len(data))

	for ; len(data) >= 32; data = data[32:] {
		s.c += getUint64(data)
		s.d += getUint64(data[8:])
		s.mix()
		s.a += getUint64(data[16:])
		s.b += getUint64(data[24:])
	}
	if len(data) >= 16 {
		s.c += getUint64(data)
		s.d += getUint64(data[8:])
		s.mix()
		data = data[16:]
	}

	s.d += l << 56
	switch len(data) {
	case 15:
		s.d += (uint64(data[14]) << 48) |
			(uint64(getUint16(data[12:])) << 32) |
			uint64(getUint32(data[8:]))
		s.c += getUint64(data)
	case 14:
		s.d += (uint64(getUint16(data[12:])) << 32) |
			uint64(getUint32(data[8:]))
		s.c += getUint64(data)
	case 13:
		s.d += (uint64(data[12]) << 32) | uint64(getUint32(data[8:]))
		s.c += getUint64(data)
	case 12:
		s.d += uint64(getUint32(data[8:]))
		s.c += getUint64(data)
	case 11:
		s.d += (uint64(data[10]) << 16) | uint64(getUint16(data[8:]))
		s.c += getUint64(data)
	case 10:
		s.d += uint64(getUint16(data[8:]))
		s.c += getUint64(data)
	case 9:
		s.d += uint64(data[8])
		s.c += getUint64(data)
	case 8:
		s.c += getUint64(data)
	case 7:
		s.c += (uint64(data[6]) << 48) |
			(uint64(getUint16(data[4:])) << 32) |
			uint64(getUint32(data))
	case 6:
		s.c += (uint64(getUint16(data[4:])) << 32) |
			uint64(getUint32(data))
	case 5:
		s.c += (uint64(data[4]) << 32) | uint64(getUint32(data))
	case 4:
		s.c += uint64(getUint32(data))
	case 3:
		s.c += (uint64(data[2]) << 16) | uint64(getUint16(data))
	case 2:
		s.c += uint64(getUint16(data))
	case 1:
		s.c += uint64(data[0])
	case 0:
		s.c += magic
		s.d += magic

	}
	s.end()
	return [2]uint64{s.a, s.b}
}

func Hash64(seed uint64, data []byte) uint64 {
	return Hash128(seed, data)[0]
}
