#ifndef PROXY_H
#define PROXY_H

#include <string>
#include <vector>
#include "proxy_types.h"

class proxy {
public:

    virtual void init(const std::vector<std::string> &keys, const std::vector<std::string> &values, void ** args) = 0;
    virtual void close() = 0;
    virtual std::string get(const std::string &key) = 0;
    virtual void put(const std::string &key, const std::string &value) = 0;
    virtual std::vector<std::string> get_batch(const std::vector<std::string> &keys) = 0;
    virtual void put_batch(const std::vector<std::string> &keys, const std::vector<std::string> &values) = 0;
    virtual std::string get(int queue_id, const std::string &key) = 0;
    virtual void put(int queue_id, const std::string &key, const std::string &value) = 0;
    virtual std::vector<std::string> get_batch(int queue_id, const std::vector<std::string> &keys) = 0;
    virtual void put_batch(int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) = 0;
    virtual std::vector<std::string> mix_batch(int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) = 0;
    // virtual void init_db(const std::vector<std::string> & keys, const std::vector<std::string> & values) =0;
    virtual void init_args(const int64_t B, const int64_t R, const int64_t F, const int64_t D, const int64_t C, const int64_t N) =0;

    virtual void async_get(const sequence_id &seq_id, const std::string &key) = 0;
    virtual void async_put(const sequence_id &seq_id, const std::string &key, const std::string &value) = 0;
    virtual void async_get_batch(const sequence_id &seq_id, const std::vector<std::string> &keys) = 0;
    virtual void async_put_batch(const sequence_id &seq_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) = 0;
    virtual void async_get(const sequence_id &seq_id, int queue_id, const std::string &key) = 0;
    virtual void async_put(const sequence_id &seq_id, int queue_id, const std::string &key, const std::string &value) = 0;
    virtual void async_get_batch(const sequence_id &seq_id, int queue_id, const std::vector<std::string> &keys) = 0;
    virtual void async_put_batch(const sequence_id &seq_id, int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) = 0;


};
#endif //PANCAKE_PROXY_H