#include "rule.h"

#include <cerrno>
#include <cstdlib>
#include <utility>

#include "experiment.pb.h"

namespace simple_abtest {

template <typename T>
static bool Compare(OpType op, T lhs, T rhs) {
  switch (op) {
    case OpType::kEqual:
      return lhs == rhs;
    case OpType::kNotEqual:
      return lhs != rhs;
    case OpType::kLessThan:
      return lhs < rhs;
    case OpType::kLessOrEqual:
      return lhs <= rhs;
    case OpType::kGreatThan:
      return lhs > rhs;
    case OpType::kGreatOrEqual:
      return lhs >= rhs;
    default:
      return false;
  }
}

static ExprNode ConvertExprNode(const proto::ExprNode& src) {
  ExprNode node;
  node.op = static_cast<OpType>(src.op());
  node.dtype = static_cast<DataType>(src.dtype());
  node.key = src.key();
  node.param_s = src.s();
  node.param_i = src.i();
  node.param_f = src.f();
  node.child.reserve(static_cast<std::size_t>(src.child_size()));
  for (std::uint32_t item : src.child()) {
    node.child.push_back(static_cast<std::size_t>(item));
  }
  for (const auto& item : src.ss()) {
    node.param_ss.emplace(item);
  }
  return node;
}

static bool ConvertExpr(const google::protobuf::RepeatedPtrField<proto::ExprNode>& src,
                        RuleExpr::Nodes* out,
                        std::string* error) {
  RuleExpr::Nodes nodes;
  nodes.reserve(static_cast<std::size_t>(src.size()));
  for (const auto& item : src) {
    nodes.push_back(ConvertExprNode(item));
  }

  std::vector<bool> used(nodes.size(), false);
  for (std::size_t i = 0; i < nodes.size(); ++i) {
    const auto& node = nodes[i];
    for (std::size_t child : node.child) {
      if (child == 0 || child >= nodes.size() || used[child]) {
        if (error != nullptr) {
          *error = "broken filter config";
        }
        return false;
      }
      used[child] = true;
    }

    switch (node.op) {
      case OpType::kAnd:
      case OpType::kOr:
        if (node.child.size() < 2 || node.dtype != DataType::kNull) {
          if (error != nullptr) {
            *error = "broken filter config";
          }
          return false;
        }
        break;
      case OpType::kNot:
        if (node.child.size() != 1 || node.dtype != DataType::kNull) {
          if (error != nullptr) {
            *error = "broken filter config";
          }
          return false;
        }
        break;
      case OpType::kIn:
      case OpType::kNotIn:
        if (!node.child.empty() || node.key.empty() || node.dtype != DataType::kStr ||
            node.param_ss.empty()) {
          if (error != nullptr) {
            *error = "broken filter config";
          }
          return false;
        }
        break;
      case OpType::kEqual:
      case OpType::kNotEqual:
      case OpType::kLessThan:
      case OpType::kLessOrEqual:
      case OpType::kGreatThan:
      case OpType::kGreatOrEqual:
        if (!node.child.empty() || node.key.empty() ||
            (node.dtype != DataType::kStr && node.dtype != DataType::kInt &&
             node.dtype != DataType::kFloat)) {
          if (error != nullptr) {
            *error = "broken filter config";
          }
          return false;
        }
        break;
      default:
        if (error != nullptr) {
          *error = "broken filter config";
        }
        return false;
    }
  }
  *out = std::move(nodes);
  return true;
}

static bool EvalNode(const RuleExpr::Nodes& expr, std::size_t index, const std::unordered_map<std::string, std::string>& args) {
  const auto& node = expr[index];
  switch (node.op) {
    case OpType::kAnd:
      for (std::size_t child : node.child) {
        if (!EvalNode(expr, child, args)) {
          return false;
        }
      }
      return true;
    case OpType::kOr:
      for (std::size_t child : node.child) {
        if (EvalNode(expr, child, args)) {
          return true;
        }
      }
      return false;
    case OpType::kNot:
      return !EvalNode(expr, node.child.front(), args);
    case OpType::kIn: {
      auto it = args.find(node.key);
      return it != args.end() && node.param_ss.count(it->second) > 0;
    }
    case OpType::kNotIn: {
      auto it = args.find(node.key);
      return it != args.end() && node.param_ss.count(it->second) == 0;
    }
    case OpType::kEqual:
    case OpType::kNotEqual:
    case OpType::kLessThan:
    case OpType::kLessOrEqual:
    case OpType::kGreatThan:
    case OpType::kGreatOrEqual: {
      auto it = args.find(node.key);
      if (it == args.end()) {
        return false;
      }
      switch (node.dtype) {
        case DataType::kStr:
          return Compare(node.op, it->second, node.param_s);
        case DataType::kInt: {
          std::int64_t parsed = 0;
          return ParseInt64(it->second, &parsed) && Compare<std::int64_t>(node.op, parsed, node.param_i);
        }
        case DataType::kFloat: {
          double parsed = 0.0;
          return ParseDouble(it->second, &parsed) && Compare(node.op, parsed, node.param_f);
        }
        case DataType::kNull:
          break;
      }
      return false;
    }
    case OpType::kNull:
      break;
  }
  return false;
}

RuleExpr::RuleExpr(Nodes nodes) : nodes_(std::move(nodes)) {}

bool RuleExpr::FromProto(const google::protobuf::RepeatedPtrField<proto::ExprNode>& src,
                         RuleExpr* out,
                         std::string* error) {
  Nodes nodes;
  if (!ConvertExpr(src, &nodes, error)) {
    return false;
  }
  *out = RuleExpr(std::move(nodes));
  return true;
}

bool RuleExpr::Eval(const std::unordered_map<std::string, std::string>& args) const {
  return nodes_.empty() || EvalNode(nodes_, 0, args);
}

bool RuleExpr::Empty() const {
  return nodes_.empty();
}

const RuleExpr::Nodes& RuleExpr::nodes() const {
  return nodes_;
}

bool EvalExpr(const RuleExpr& expr, const std::unordered_map<std::string, std::string>& args) {
  return expr.Eval(args);
}

bool ParseInt64(const std::string& input, std::int64_t* out) {
  if (out == nullptr || input.empty()) {
    return false;
  }
  char* end = nullptr;
  errno = 0;
  long long value = std::strtoll(input.c_str(), &end, 10);
  if (errno != 0 || end == input.c_str() || *end != '\0') {
    return false;
  }
  *out = static_cast<std::int64_t>(value);
  return true;
}

bool ParseDouble(const std::string& input, double* out) {
  if (out == nullptr || input.empty()) {
    return false;
  }
  char* end = nullptr;
  errno = 0;
  double value = std::strtod(input.c_str(), &end);
  if (errno != 0 || end == input.c_str() || *end != '\0') {
    return false;
  }
  *out = value;
  return true;
}

}  // namespace simple_abtest
