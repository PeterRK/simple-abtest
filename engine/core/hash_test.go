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
	expected := [][2]uint64{
		[2]uint64{0x232706fc6bf50919, 0x8b72ee65b4e851c7},
		[2]uint64{0x50209687d54ec67e, 0x62fe85108df1cf6d},
		[2]uint64{0xfbe67d8368f3fb4f, 0xb54a5a89706d5a5a},
		[2]uint64{0x2882d11a5846ccfa, 0x6b21b0e870109222},
		[2]uint64{0xf5e0d56325d6d000, 0xaf8703c9f9ac75e5},
		[2]uint64{0x59a0f67b7ae7a5ad, 0x84d7aeabc053b848},
		[2]uint64{0xf01562a268e42c21, 0xdfe994ab22873e7e},
		[2]uint64{0x16133104620725dd, 0xa5ca36afa7182e6a},
		[2]uint64{0x7a9378dcdf599479, 0x30f5a569a74ecdd7},
		[2]uint64{0xd9f07bdc76c20a78, 0x34f0621847f7888a},
		[2]uint64{0x332a4fff07df83da, 0xfa40557cc0ea6b72},
		[2]uint64{0x976beeefd11659dc, 0x8a3187b6a72d0039},
		[2]uint64{0xc3fcc139e4c6832a, 0xdadfeff6e01e2f2e},
		[2]uint64{0x86130593c7746a6f, 0x8ac9fb14904fe39d},
		[2]uint64{0x70550dbe5cdde280, 0xddb95757282706c0},
		[2]uint64{0x67211fbaf6b9122d, 0x68f4e8f3bbc700db},
		[2]uint64{0xe2d06846964b80ad, 0x6005068ac75c4c20},
		[2]uint64{0xd55b3c010258ce93, 0x981c8b03659d9950},
		[2]uint64{0x5a2507daa032fa13, 0x0d1c989bfc0c6cf7},
		[2]uint64{0xaf8618678ae5cd55, 0xe0b75cfad427eefc},
		[2]uint64{0xad5a7047e8a139d8, 0x183621cf988a753e},
		[2]uint64{0x8fc110192723cd5e, 0x203129f80764b844},
		[2]uint64{0x50170b4485d7af19, 0x7f2c79d145db7d35},
		[2]uint64{0x7c32444652212bf3, 0x27fd51b9156e2ad2},
		[2]uint64{0x90e571225cce7360, 0xf743b8f6f7433428},
		[2]uint64{0x9919537c1add41e1, 0x7ff0158f05b261f2},
		[2]uint64{0x3a70a8070883029f, 0xc5dcba911815d20a},
		[2]uint64{0xcc32b418290e2879, 0xbb7945d6d79b5dfb},
		[2]uint64{0xde493e4646077aeb, 0x465c2ea52660973a},
		[2]uint64{0x4d3ad9b55316f970, 0x9137e3040a7d87bb},
		[2]uint64{0x1547de75efe848f4, 0x21ae3f08b5330aac},
		[2]uint64{0xe2ead0cc6aab6aff, 0x29a20bccf77e70a7},
		[2]uint64{0x3dc2f4a9e9b451b4, 0x27de306dde7b60d2},
		[2]uint64{0xce247654a4de9f51, 0x040097e45e948d66},
		[2]uint64{0xbc118f2ba2305503, 0x810f05d0ea32853f},
		[2]uint64{0xb55cd8bdcac2a118, 0x4e93b65164705d2a},
		[2]uint64{0xb7c97db807c32f38, 0x510723230adef63d},
	}

	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	for i := 0; i < len(expected); i++ {
		w := Hash128(0, data[:i])
		assert(t, w[0] == expected[i][0] && w[1] == expected[i][1])
	}
}
