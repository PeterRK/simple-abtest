#include "simple_abtest/sdk.h"

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <ctime>
#include <memory>
#include <mutex>
#include <sstream>
#include <thread>
#include <utility>
#include <vector>

#include <curl/curl.h>

#include "model.h"

namespace simple_abtest {
namespace {

struct CurlInit {
  CurlInit() {
    curl_global_init(CURL_GLOBAL_DEFAULT);
  }

  ~CurlInit() {
    curl_global_cleanup();
  }
};

CurlInit& GetCurlInit() {
  static CurlInit init;
  return init;
}

std::size_t WriteBody(char* ptr, std::size_t size, std::size_t nmemb, void* userdata) {
  auto* body = static_cast<std::string*>(userdata);
  body->append(ptr, size * nmemb);
  return size * nmemb;
}

std::string JoinUrl(const std::string& address, std::uint32_t appid) {
  if (!address.empty() && address.back() == '/') {
    return address.substr(0, address.size() - 1) + "/app/" + std::to_string(appid);
  }
  return address + "/app/" + std::to_string(appid);
}

bool HttpGet(const std::string& url,
             const std::string& token,
             std::string* response,
             std::string* error) {
  (void)GetCurlInit();

  CURL* curl = curl_easy_init();
  if (curl == nullptr) {
    if (error != nullptr) {
      *error = "curl_easy_init failed";
    }
    return false;
  }

  std::string token_header = "ACCESS_TOKEN: " + token;
  struct curl_slist* headers = nullptr;
  headers = curl_slist_append(headers, token_header.c_str());

  curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
  curl_easy_setopt(curl, CURLOPT_HTTPGET, 1L);
  curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
  curl_easy_setopt(curl, CURLOPT_ACCEPT_ENCODING, "");
  curl_easy_setopt(curl, CURLOPT_TIMEOUT, 10L);
  curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, WriteBody);
  curl_easy_setopt(curl, CURLOPT_WRITEDATA, response);

  CURLcode rc = curl_easy_perform(curl);
  long status = 0;
  curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &status);

  curl_slist_free_all(headers);
  curl_easy_cleanup(curl);

  if (rc != CURLE_OK) {
    if (error != nullptr) {
      *error = curl_easy_strerror(rc);
    }
    return false;
  }
  if (status != 200) {
    if (error != nullptr) {
      std::ostringstream oss;
      oss << "fetch app info failed: status=" << status;
      *error = oss.str();
    }
    return false;
  }
  return true;
}

class ClientImpl final : public Client {
 public:
  ClientImpl(std::string url, std::string token, std::chrono::seconds ttl)
      : src_url_(std::move(url)), access_token_(std::move(token)), ttl_(ttl) {}

  ~ClientImpl() override {
    StopRefreshLoop();
  }

  bool Refresh(std::string* error) override {
    std::string payload;
    if (!HttpGet(src_url_, access_token_, &payload, error)) {
      return false;
    }

    auto exps = std::make_shared<std::vector<Experiment>>();
    if (!ParseExperimentsJson(payload, exps.get(), error)) {
      return false;
    }

    {
      std::lock_guard<std::mutex> lock(data_mu_);
      data_ = std::move(exps);
    }
    stamp_.store(static_cast<std::uint32_t>(std::time(nullptr)));
    return true;
  }

  std::uint32_t Stamp() const override {
    return stamp_.load();
  }

  Decision AB(const std::string& key, const std::unordered_map<std::string, std::string>& ctx) const override {
    std::shared_ptr<std::vector<Experiment>> snapshot;
    {
      std::lock_guard<std::mutex> lock(data_mu_);
      snapshot = data_;
    }
    if (!snapshot) {
      return {};
    }
    return GetExpConfig(*snapshot, key, ctx);
  }

  void StartRefreshLoop() {
    if (ttl_.count() == 0) {
      return;
    }
    active_.store(true);
    refresh_thread_ = std::thread(&ClientImpl::RefreshLoop, this);
  }

 private:
  void StopRefreshLoop() {
    if (active_.exchange(false)) {
      stop_cv_.notify_all();
    }
    if (refresh_thread_.joinable()) {
      refresh_thread_.join();
    }
  }

  void RefreshLoop() {
    while (active_.load()) {
      std::unique_lock<std::mutex> lock(stop_mu_);
      if (stop_cv_.wait_for(lock, ttl_, [this]() { return !active_.load(); })) {
        break;
      }
      lock.unlock();
      Refresh(nullptr);
    }
  }

  std::string src_url_;
  std::string access_token_;
  std::chrono::seconds ttl_;

  mutable std::mutex data_mu_;
  std::shared_ptr<std::vector<Experiment>> data_;
  std::atomic<std::uint32_t> stamp_{0};
  std::atomic<bool> active_{false};

  mutable std::mutex stop_mu_;
  std::condition_variable stop_cv_;
  std::thread refresh_thread_;
};

}  // namespace

std::unique_ptr<Client> Client::Create(const std::string& address, const std::string& access_token,
                                       std::uint32_t appid, std::uint32_t ttl,  std::string* error) {
  if (ttl > 0 && ttl <= 60) {
    ttl = 60;
  }

  auto client = std::make_unique<ClientImpl>(JoinUrl(address, appid), access_token,
                                             std::chrono::seconds(ttl));
  if (!client->Refresh(error)) {
    return nullptr;
  }

  client->StartRefreshLoop();
  return client;
}

}  // namespace simple_abtest
