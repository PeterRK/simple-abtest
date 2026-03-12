package core

import (
	"testing"
)

func assert(t *testing.T, state bool) {
	if !state {
		t.FailNow()
	}
}

func TestHash(t *testing.T) {
	expected := []uint64{
		0x232706fc6bf50919,
		0x50209687d54ec67e,
		0xfbe67d8368f3fb4f,
		0x2882d11a5846ccfa,
		0xf5e0d56325d6d000,
		0x59a0f67b7ae7a5ad,
		0xf01562a268e42c21,
		0x16133104620725dd,
		0x7a9378dcdf599479,
		0xd9f07bdc76c20a78,
		0x332a4fff07df83da,
		0x976beeefd11659dc,
		0xc3fcc139e4c6832a,
		0x86130593c7746a6f,
		0x70550dbe5cdde280,
		0x67211fbaf6b9122d,
		0xe2d06846964b80ad,
		0xd55b3c010258ce93,
		0x5a2507daa032fa13,
		0xaf8618678ae5cd55,
		0xad5a7047e8a139d8,
		0x8fc110192723cd5e,
		0x50170b4485d7af19,
		0x7c32444652212bf3,
		0x90e571225cce7360,
		0x9919537c1add41e1,
		0x3a70a8070883029f,
		0xcc32b418290e2879,
		0xde493e4646077aeb,
		0x4d3ad9b55316f970,
		0x1547de75efe848f4,
		0xe2ead0cc6aab6aff,
		0x3dc2f4a9e9b451b4,
		0xce247654a4de9f51,
		0xbc118f2ba2305503,
		0xb55cd8bdcac2a118,
		0xb7c97db807c32f38,
	}

	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	for i := 0; i < len(expected); i++ {
		w := Hash(0, data[:i])
		assert(t, w == expected[i])
	}
}
