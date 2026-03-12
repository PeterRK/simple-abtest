cmake_minimum_required(VERSION 3.16)
project(simple_abtest_cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(CURL REQUIRED)
find_package(ZLIB REQUIRED)
find_package(Protobuf REQUIRED)
find_package(GTest REQUIRED)

set(PROTO_FILES ${CMAKE_CURRENT_SOURCE_DIR}/proto/experiment.proto)
protobuf_generate_cpp(PROTO_SRCS PROTO_HDRS ${PROTO_FILES})

add_library(simple_abtest_sdk
  ${PROTO_SRCS}
  src/hash.cc
  src/rule.cc
  src/model.cc
  src/sdk.cc
)

target_include_directories(simple_abtest_sdk
  PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
  PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/src
    ${CMAKE_CURRENT_BINARY_DIR}
)

target_link_libraries(simple_abtest_sdk
  PUBLIC
    CURL::libcurl
    ZLIB::ZLIB
    protobuf::libprotobuf
)

target_compile_options(simple_abtest_sdk PRIVATE -Wall -Wextra -Wpedantic)

enable_testing()

add_executable(simple_abtest_sdk_tests
  tests/core_test.cc
)

target_include_directories(simple_abtest_sdk_tests PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
target_link_libraries(simple_abtest_sdk_tests
  PRIVATE
    simple_abtest_sdk
    GTest::gtest_main
)

include(GoogleTest)
gtest_discover_tests(simple_abtest_sdk_tests)
