#include "model.h"

#include <google/protobuf/util/json_util.h>

#include "experiment.pb.h"
#include "hash.h"

namespace simple_abtest {

static Group ConvertGroup(const proto::Group& src) {
  Group group;
  group.name = src.name();
  group.bitmap.assign(src.bm().begin(), src.bm().end());
  group.config = src.cfg();
  return group;
}

static Segment ConvertSegment(const proto::Segment& src) {
  Segment segment;
  segment.range.begin = src.r().a();
  segment.range.end = src.r().b();
  segment.seed = src.seed();
  segment.groups.reserve(static_cast<std::size_t>(src.grp_size()));
  for (const auto& item : src.grp()) {
    segment.groups.push_back(ConvertGroup(item));
  }
  return segment;
}

static Layer ConvertLayer(const proto::Layer& src) {
  Layer layer;
  layer.name = src.name();
  layer.segments.reserve(static_cast<std::size_t>(src.seg_size()));
  for (const auto& item : src.seg()) {
    layer.segments.push_back(ConvertSegment(item));
  }
  for (const auto& item : src.force_hit()) {
    layer.force_hit.emplace(item.first, HitIndex{item.second.s(), item.second.g()});
  }
  return layer;
}

static bool ConvertExperiment(const proto::Experiment& src, Experiment* out, std::string* error) {
  Experiment exp;
  if (!RuleExpr::FromProto(src.filter(), &exp.filter, error)) {
    return false;
  }
  exp.seed = src.seed();
  exp.layers.reserve(static_cast<std::size_t>(src.lyr_size()));
  for (const auto& item : src.lyr()) {
    exp.layers.push_back(ConvertLayer(item));
  }
  *out = std::move(exp);
  return true;
}

const Group* Segment::Locate(const std::string& key) const {
  std::uint64_t slot = Hash(seed, key) % 1000;
  std::size_t blk = static_cast<std::size_t>(slot >> 3);
  std::uint8_t mask = static_cast<std::uint8_t>(1U << (slot & 7));
  for (const auto& group : groups) {
    if (blk < group.bitmap.size() && (group.bitmap[blk] & mask) != 0) {
      return &group;
    }
  }
  return nullptr;
}

bool ParseExperimentsJson(const std::string& payload,
                          std::vector<Experiment>* exps,
                          std::string* error) {
  proto::ExperimentList exps_msg;
  std::string wrapped = "{\"items\":" + payload + "}";
  auto status = google::protobuf::util::JsonStringToMessage(wrapped, &exps_msg);
  if (!status.ok()) {
    if (error != nullptr) {
      *error = "invalid experiment payload: " + std::string(status.message());
    }
    return false;
  }

  std::vector<Experiment> parsed;
  parsed.reserve(static_cast<std::size_t>(exps_msg.items_size()));
  for (const auto& item : exps_msg.items()) {
    Experiment exp;
    if (!ConvertExperiment(item, &exp, error)) {
      return false;
    }
    parsed.push_back(std::move(exp));
  }
  *exps = std::move(parsed);
  return true;
}

Decision GetExpConfig(const std::vector<Experiment>& exps, const std::string& key,
                      const std::unordered_map<std::string, std::string>& ctx) {
  Decision result;
  auto mark = [&result](const Layer& layer, const Group& group) {
    result.config[layer.name] = group.config;
    result.tags.push_back(layer.name + ":" + group.name);
  };

  for (const auto& exp : exps) {
    if (!EvalExpr(exp.filter, ctx)) {
      continue;
    }

    if (exp.layers.size() == 1 && exp.layers.front().segments.size() == 1) {
      const auto& layer = exp.layers.front();
      auto hit = layer.force_hit.find(key);
      if (hit != layer.force_hit.end() &&
          hit->second.seg < layer.segments.size() &&
          hit->second.grp < layer.segments[hit->second.seg].groups.size()) {
        mark(layer, layer.segments[hit->second.seg].groups[hit->second.grp]);
        continue;
      }
      if (const Group* group = layer.segments.front().Locate(key); group != nullptr) {
        mark(layer, *group);
      }
      continue;
    }

    std::uint32_t slot = static_cast<std::uint32_t>(Hash(exp.seed, key) % 100);
    for (const auto& layer : exp.layers) {
      auto hit = layer.force_hit.find(key);
      if (hit != layer.force_hit.end() &&
          hit->second.seg < layer.segments.size() &&
          hit->second.grp < layer.segments[hit->second.seg].groups.size()) {
        mark(layer, layer.segments[hit->second.seg].groups[hit->second.grp]);
        continue;
      }
      for (const auto& seg : layer.segments) {
        if (seg.range.begin <= slot && slot < seg.range.end) {
          if (const Group* group = seg.Locate(key); group != nullptr) {
            mark(layer, *group);
          }
          break;
        }
      }
    }
  }
  return result;
}

}  // namespace simple_abtest
