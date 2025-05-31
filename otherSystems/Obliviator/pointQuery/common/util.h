#ifndef DISTRIBUTED_SGX_SORT_COMMON_UTIL_H
#define DISTRIBUTED_SGX_SORT_COMMON_UTIL_H

#include <stdint.h>
#include <stddef.h>
#include <limits.h>
#include <time.h>

static inline unsigned long long next_pow2ll(unsigned long long x) {
#ifdef __GNUC__
    unsigned long long next =
        1llu << (sizeof(x) * CHAR_BIT - __builtin_clzll(x) - 1);
    if (next < x) {
        next <<= 1;
    }
    return next;
#else
    unsigned long long next = 1;
    while (next < x) {
        next <<= 1;
    }
    return next;
#endif
}

static inline unsigned long long log2ll(unsigned long long x) {
#ifdef __GNUC__
    return sizeof(x) * CHAR_BIT - __builtin_clzll(x) - 1;
#else
    unsigned long long log = -1;
    while (x) {
        log++;
        x >>= 1;
    }
    return log;
#endif
}

static inline int comp_ul(const void *a_, const void *b_) {
    const size_t *a = a_;
    const size_t *b = b_;
    return (*a > *b) - (*a < *b);
}

static inline double get_time_difference(struct timespec *start,
        struct timespec *end) {
    return (double) (end->tv_sec * 1000000000 + end->tv_nsec
            - (start->tv_sec * 1000000000 + start->tv_nsec))
        / 1000000000;
}

static inline uint64_t htonll(uint64_t hostll) {
    union {
        unsigned char bytes[sizeof(uint64_t)];
        uint64_t t;
    } u;
    u.bytes[0] = (hostll >> (CHAR_BIT * 7)) & UCHAR_MAX;
    u.bytes[1] = (hostll >> (CHAR_BIT * 6)) & UCHAR_MAX;
    u.bytes[2] = (hostll >> (CHAR_BIT * 5)) & UCHAR_MAX;
    u.bytes[3] = (hostll >> (CHAR_BIT * 4)) & UCHAR_MAX;
    u.bytes[4] = (hostll >> (CHAR_BIT * 3)) & UCHAR_MAX;
    u.bytes[5] = (hostll >> (CHAR_BIT * 2)) & UCHAR_MAX;
    u.bytes[6] = (hostll >> CHAR_BIT) & UCHAR_MAX;
    u.bytes[7] = hostll & UCHAR_MAX;
    return u.t;
}

static inline uint64_t ntohll(uint64_t netll) {
    union {
        unsigned char bytes[sizeof(uint64_t)];
        uint64_t t;
    } u = {
        .t = netll,
    };
    return ((uint64_t) u.bytes[0]) << (CHAR_BIT * 7)
        | ((uint64_t) u.bytes[1]) << (CHAR_BIT * 6)
        | ((uint64_t) u.bytes[2]) << (CHAR_BIT * 5)
        | ((uint64_t) u.bytes[3]) << (CHAR_BIT * 4)
        | ((uint64_t) u.bytes[4]) << (CHAR_BIT * 3)
        | ((uint64_t) u.bytes[5]) << (CHAR_BIT * 2)
        | ((uint64_t) u.bytes[6]) << CHAR_BIT
        | u.bytes[7];
}

void *bsearch_ge(const void *key, const void *arr, size_t num_elems,
        size_t elem_size, int (*comparator)(const void *a, const void *b));


// Returns log2 rounded up.
uint64_t calculatelog2(uint64_t value);

// Returns log2 floor.
uint64_t calculatelog2_floor(uint64_t value);

// Returns largest power of two less than N
uint64_t pow2_lt(uint64_t N);

// Returns largest power of two greater than N
uint64_t pow2_gt(uint64_t N);

int printf(const char* fmt, ...);

#endif /* distributed-sgx-sort/common/util.h */
