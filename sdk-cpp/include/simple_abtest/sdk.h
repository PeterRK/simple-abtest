#pragma once

#include <chrono>
#include <cstdint>
#include <memory>
#include <string>
#include <unordered_map>
#include <vector>

namespace simple_abtest {

struct Decision {
  std::unordered_map<std::string, std::string> config;
  std::vector<std::string> tags;
};

class Client {
 public:
  static std::unique_ptr<Client> Create(const std::string& address, const std::string& access_token,
                                        std::uint32_t appid, std::uint32_t ttl, std::string* error = nullptr);

  virtual ~Client() = default;

  virtual bool Refresh(std::string* error = nullptr) = 0;
  virtual std::uint32_t Stamp() const = 0;
  virtual Decision AB(const std::string& key, const std::unordered_map<std::string, std::string>& ctx) const = 0;
};

}  // namespace simple_abtest
