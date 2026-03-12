#pragma once
#ifndef SIMPLE_ABTEST_HASH_H
#define SIMPLE_ABTEST_HASH_H

#include <cstdint>
#include <string>

namespace simple_abtest {

extern uint64_t Hash(uint64_t seed, const uint8_t* msg, unsigned len) noexcept;

static inline uint64_t Hash(uint64_t seed, const std::string& data) noexcept {
	return Hash(seed, reinterpret_cast<const uint8_t*>(data.c_str()), data.length());
}

} //simple_abtest
#endif // SIMPLE_ABTEST_HASH_H