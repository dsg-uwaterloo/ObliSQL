#ifndef DISTRIBUTED_SGX_SORT_COMMON_DEFS_H
#define DISTRIBUTED_SGX_SORT_COMMON_DEFS_H

#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define MAX(a, b) ((a) > (b) ? (a) : (b))
#define CEIL_DIV(a, b) (((a) + (b) - 1) / (b))
#define ROUND_DOWN(a, b) ((a) / (b) * (b))
#define ROUND_UP(a, b) (((a) + (b) - 1) / (b) * (b))
#define UNUSED __attribute__((unused))
#define PACKED __attribute__((packed))

#endif /* distributed-sgx-sort/common/defs.h */
