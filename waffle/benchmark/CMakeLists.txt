set(STORAGE_INCLUDE ${CMAKE_CURRENT_SOURCE_DIR}/../libstorage/src)
set(CLIENT_INCLUDE ${CMAKE_CURRENT_SOURCE_DIR}/../libclient/src)
set(PROXY_INCLUDE ${CMAKE_CURRENT_SOURCE_DIR}/../proxy/src)
set(SERVICE_INCLUDE ${CMAKE_CURRENT_SOURCE_DIR}/../service/src)
set(BENCHMARK_INCLUDE ${CMAKE_CURRENT_SOURCE_DIR}/src)

include_directories(${STORAGE_INCLUDE} ${CLIENT_INCLUDE} ${PROXY_INCLUDE} ${BENCHMARK_INCLUDE} ${SERVICE_INCLUDE} )

# set(opentelemetry-cpp_DIR /usr/local/lib/cmake/opentelemetry-cpp)
# find_package(absl CONFIG)
# find_package(protobuf)
# find_package(opentelemetry-cpp CONFIG REQUIRED NO_DEFAULT_PATH)

add_executable(proxy_benchmark
        src/proxy_benchmark.cpp

        ../libclient/src/client.h
        ../libclient/src/proxy_client.cpp
        ../libclient/src/proxy_client.h
        ../libclient/src/async_proxy_client.cpp
        ../libclient/src/async_proxy_client.h
        ../libclient/src/command_response_reader.cpp
        ../libclient/src/command_response_reader.h

        ../service/src/thrift_server.cpp
        ../service/src/thrift_server.h
        ../service/src/thrift_handler_factory.cpp
        ../service/src/thrift_handler_factory.h
        ../service/src/thrift_handler.cpp
        ../service/src/thrift_handler.h
        ../service/src/thrift_response_client_map.cpp
        ../service/src/thrift_response_client_map.h
        ../service/src/thrift_response_client.cpp
        ../service/src/thrift_response_client.h
        ../service/src/waffle_thrift_response.cpp
        ../service/src/waffle_thrift_response.h
        ../service/src/waffle_thrift.cpp
        ../service/src/waffle_thrift.h
        ../service/src/proxy_types.cpp
        ../service/src/proxy_types.h

        ../proxy/src/waffle_proxy.cpp
        ../proxy/src/waffle_proxy.h
        ../proxy/src/proxy.h
        ../proxy/src/operation.h
        ../proxy/src/queue.h
        ../proxy/src/util.h
        ../proxy/src/util.cpp
        ../proxy/src/Cache.cpp
        ../proxy/src/Cache.hpp
        ../proxy/src/FrequencySmoother.cpp
        ../proxy/src/FrequencySmoother.hpp
        ../proxy/src/encryption_engine.cpp
        ../proxy/src/encryption_engine.h
        ../proxy/src/ThreadSafeUnorderedMap.h
        ../proxy/src/evictedItems.cpp
        ../proxy/src/evictedItems.h

        ../libstorage/src/storage_interface.h
        ../libstorage/src/redis.h
        ../libstorage/src/redis.cpp
        #../libstorage/src/memcached.cpp
        #../libstorage/src/memcached.h
        ../service/src/thrift_utils.h)
add_dependencies(proxy_benchmark cpp_redis libcuckoo openssl_ep thrift_ep)
target_link_libraries(proxy_benchmark
        ${THRIFTNB_LIBRARY}
        ${THRIFT_LIBRARY}
        ${LIBEVENT_LIBRARY}
        ${CPP_REDIS_LIBRARIES}
        ${OPENSSL_LIBRARIES}
        ${SSDB_CLIENT_LIBRARY}
        ${LEVELDB_LIBRARY}
        ${SNAPPY_LIBRARY}
        ${JEMALLOC_LIBRARY}
        ${CMAKE_DL_LIBS}
        ${ZLIB_LIBRARIES}
        ${BZIP2_LIBRARIES}
        #${LIBCUCKOO_LIBRARIES}
        #${LIBMEMCACHED_LIBRARIES}
        ${PROXY_INCLUDE_LIBRARIES}
        ${STORAGE_INCLUDE_LIBRARIES}
        ${BENCHMARK_INCLUDE_LIBRARIES}
        ${SERVICE_INCLUDE_LIBRARIES}
        ${CLIENT_INCLUDE_LIBRARIES}
        # opentelemetry_common
        # opentelemetry_resources
        # opentelemetry_exporter_otlp_grpc
        # opentelemetry_trace
        Threads::Threads)
