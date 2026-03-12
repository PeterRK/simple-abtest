#pragma once

#include <cstdint>
#include <string>
#include <unordered_set>
#include <vector>

#include <google/protobuf/repeated_ptr_field.h>

#include "simple_abtest/sdk.h"

namespace simple_abtest {
namespace proto {
class ExprNode;
}

enum class OpType : std::uint32_t {
  kNull = 0,
  kAnd = 1,
  kOr = 2,
  kNot = 3,
  kIn = 4,
  kNotIn = 5,
  kEqual = 6,
  kNotEqual = 7,
  kLessThan = 8,
  kLessOrEqual = 9,
  kGreatThan = 10,
  kGreatOrEqual = 11,
};

enum class DataType : std::uint32_t {
  kNull = 0,
  kStr = 1,
  kInt = 2,
  kFloat = 3,
};

struct ExprNode {
  OpType op = OpType::kNull;
  DataType dtype = DataType::kNull;
  std::string key;
  std::string param_s;
  std::int64_t param_i = 0;
  double param_f = 0.0;
  std::vector<std::size_t> child;
  std::unordered_set<std::string> param_ss;
};

class RuleExpr {
 public:
  using Nodes = std::vector<ExprNode>;

  RuleExpr() = default;
  explicit RuleExpr(Nodes nodes);

  static bool FromProto(const google::protobuf::RepeatedPtrField<proto::ExprNode>& src,
                        RuleExpr* out,
                        std::string* error);

  bool Eval(const std::unordered_map<std::string, std::string>& args) const;
  bool Empty() const;
  const Nodes& nodes() const;

 private:
  Nodes nodes_;
};

bool EvalExpr(const RuleExpr& expr, const std::unordered_map<std::string, std::string>& args);
bool ParseInt64(const std::string& input, std::int64_t* out);
bool ParseDouble(const std::string& input, double* out);

}  // namespace simple_abtest
