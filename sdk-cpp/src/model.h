#pragma once

#include <cstdint>
#include <string>
#include <unordered_map>
#include <vector>

#include "rule.h"
#include "simple_abtest/sdk.h"

namespace simple_abtest {

struct Group {
  std::string name;
  std::vector<std::uint8_t> bitmap;
  std::string config;
};

struct SegmentRange {
  std::uint32_t begin = 0;
  std::uint32_t end = 0;
};

struct Segment {
  SegmentRange range;
  std::uint32_t seed = 0;
  std::vector<Group> groups;

  const Group* Locate(const std::string& key) const;
};

struct HitIndex {
  std::uint32_t seg = 0;
  std::uint32_t grp = 0;
};

struct Layer {
  std::string name;
  std::vector<Segment> segments;
  std::unordered_map<std::string, HitIndex> force_hit;
};

struct Experiment {
  RuleExpr filter;
  std::uint32_t seed = 0;
  std::vector<Layer> layers;
};

bool ParseExperimentsJson(const std::string& payload,
                          std::vector<Experiment>* exps,
                          std::string* error);
Decision GetExpConfig(const std::vector<Experiment>& exps, const std::string& key,
                      const std::unordered_map<std::string, std::string>& ctx);

}  // namespace simple_abtest
