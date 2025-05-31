#ifndef DISTRIBUTED_SGX_SORT_COMMON_ALGORITHM_TYPE_H
#define DISTRIBUTED_SGX_SORT_COMMON_ALGORITHM_TYPE_H

enum algorithm_type {
    SORT_UNSET = 0,
    SORT_BITONIC,
    SORT_BUCKET,
    SORT_OPAQUE,
    SORT_ORSHUFFLE,
    SCALABLE_OBLIVIOUS_JOIN,
};

#endif /* distributed-sgx-sort/common/algorithm_type.h */
