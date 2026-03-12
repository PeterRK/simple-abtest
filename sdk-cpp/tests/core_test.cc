#include <cstdint>
#include <utility>
#include <string>
#include <vector>

#include <gtest/gtest.h>

#include "hash.h"
#include "model.h"

namespace simple_abtest {
namespace {

std::vector<std::uint8_t> FullBitmap() {
  return std::vector<std::uint8_t>(125, 0xFF);
}

std::string EncodeBase64(const std::vector<std::uint8_t>& data) {
  static const char kTable[] =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::string out;
  int val = 0;
  int valb = -6;
  for (std::uint8_t c : data) {
    val = (val << 8) + c;
    valb += 8;
    while (valb >= 0) {
      out.push_back(kTable[(val >> valb) & 0x3F]);
      valb -= 6;
    }
  }
  if (valb > -6) {
    out.push_back(kTable[((val << 8) >> (valb + 8)) & 0x3F]);
  }
  while (out.size() % 4 != 0) {
    out.push_back('=');
  }
  return out;
}

std::string FindKey(std::uint32_t seed, std::uint64_t begin, std::uint64_t end) {
  for (int i = 0; i < 10000; ++i) {
    std::string key = "k" + std::to_string(i);
    std::uint64_t slot = Hash(seed, key) % 100;
    if (slot >= begin && slot < end) {
      return key;
    }
  }
  return "";
}

TEST(HashTest, MatchesGoImplementation) {
  const std::vector<std::uint64_t> expected = {
      0x232706fc6bf50919ULL, 0x50209687d54ec67eULL, 0xfbe67d8368f3fb4fULL,
      0x2882d11a5846ccfaULL, 0xf5e0d56325d6d000ULL, 0x59a0f67b7ae7a5adULL,
      0xf01562a268e42c21ULL, 0x16133104620725ddULL, 0x7a9378dcdf599479ULL,
      0xd9f07bdc76c20a78ULL, 0x332a4fff07df83daULL, 0x976beeefd11659dcULL,
      0xc3fcc139e4c6832aULL, 0x86130593c7746a6fULL, 0x70550dbe5cdde280ULL,
      0x67211fbaf6b9122dULL, 0xe2d06846964b80adULL, 0xd55b3c010258ce93ULL,
      0x5a2507daa032fa13ULL, 0xaf8618678ae5cd55ULL, 0xad5a7047e8a139d8ULL,
      0x8fc110192723cd5eULL, 0x50170b4485d7af19ULL, 0x7c32444652212bf3ULL,
      0x90e571225cce7360ULL, 0x9919537c1add41e1ULL, 0x3a70a8070883029fULL,
      0xcc32b418290e2879ULL, 0xde493e4646077aebULL, 0x4d3ad9b55316f970ULL,
      0x1547de75efe848f4ULL, 0xe2ead0cc6aab6affULL, 0x3dc2f4a9e9b451b4ULL,
      0xce247654a4de9f51ULL, 0xbc118f2ba2305503ULL, 0xb55cd8bdcac2a118ULL,
      0xb7c97db807c32f38ULL,
  };

  const std::string data = "0123456789abcdefghijklmnopqrstuvwxyz";
  ASSERT_EQ(expected.size(), data.size() + 1);

  for (std::size_t i = 0; i < expected.size(); ++i) {
    EXPECT_EQ(Hash(0, data.substr(0, i)), expected[i]) << "prefix_len=" << i;
  }
}

TEST(RuleTest, EvalLogic) {
  const std::string bm = EncodeBase64(FullBitmap());
  std::vector<Experiment> exps;
  std::string error;
  const std::string payload = std::string(R"json([
        {
          "filter": [
            {"op": 1, "child": [1, 2]},
            {"op": 6, "dtype": 1, "key": "country", "s": "CN"},
            {"op": 10, "dtype": 2, "key": "age", "i": 18}
          ],
          "lyr": [{
            "name": "L1",
            "seg": [{
              "seed": 1,
              "grp": [{
                "name": "A",
                "bm": ")json") +
          bm +
          R"json(",
                "cfg": "cfgA"
              }]
            }]
          }]
        }
      ])json";
  const bool parsed = ParseExperimentsJson(payload, &exps, &error);
  ASSERT_TRUE(parsed) << error;

  EXPECT_TRUE(EvalExpr(exps.front().filter, {{"country", "CN"}, {"age", "20"}}));
  EXPECT_FALSE(EvalExpr(exps.front().filter, {{"country", "US"}, {"age", "20"}}));
  EXPECT_FALSE(EvalExpr(exps.front().filter, {{"country", "CN"}, {"age", "x"}}));
}

TEST(DispatchTest, RichSegmentAndForceHit) {
  std::string key = FindKey(1, 50, 100);
  ASSERT_FALSE(key.empty());

  Experiment exp;
  exp.seed = 1;

  Layer layer;
  layer.name = "layer1";

  Segment seg_a;
  seg_a.range.begin = 0;
  seg_a.range.end = 50;
  seg_a.seed = 1;
  seg_a.groups.push_back(Group{"A", FullBitmap(), "cfgA"});

  Segment seg_b;
  seg_b.range.begin = 50;
  seg_b.range.end = 100;
  seg_b.seed = 2;
  seg_b.groups.push_back(Group{"B", FullBitmap(), "cfgB"});
  seg_b.groups.push_back(Group{"F", FullBitmap(), "cfgForce"});

  layer.segments = {seg_a, seg_b};
  layer.force_hit["forced"] = HitIndex{1, 1};
  exp.layers.push_back(layer);

  auto result = GetExpConfig({exp}, key, {});
  ASSERT_EQ(result.config["layer1"], "cfgB");
  ASSERT_EQ(result.tags.size(), 1U);
  EXPECT_EQ(result.tags[0], "layer1:B");

  result = GetExpConfig({exp}, "forced", {});
  ASSERT_EQ(result.config["layer1"], "cfgForce");
  ASSERT_EQ(result.tags.size(), 1U);
  EXPECT_EQ(result.tags[0], "layer1:F");
}

TEST(ParserTest, DecodesBitmapAndDispatches) {
  const std::string bm = EncodeBase64(FullBitmap());
  std::vector<Experiment> exps;
  std::string error;
  const std::string payload = std::string(R"json([
        {
          "lyr": [{
            "name": "L1",
            "seg": [{
              "seed": 1,
              "grp": [{
                "name": "G1",
                "bm": ")json") +
          bm +
          R"json(",
                "cfg": "cfg1"
              }]
            }]
          }]
        }
      ])json";
  const bool parsed = ParseExperimentsJson(payload, &exps, &error);
  ASSERT_TRUE(parsed) << error;

  ASSERT_EQ(exps.size(), 1U);
  ASSERT_EQ(exps[0].layers.size(), 1U);
  ASSERT_EQ(exps[0].layers[0].segments.size(), 1U);
  EXPECT_EQ(exps[0].layers[0].segments[0].groups[0].bitmap.size(), 125U);

  auto result = GetExpConfig(exps, "user", {});
  EXPECT_EQ(result.config["L1"], "cfg1");
  ASSERT_EQ(result.tags.size(), 1U);
  EXPECT_EQ(result.tags[0], "L1:G1");
}

}  // namespace
}  // namespace simple_abtest
