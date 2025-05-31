#include "common/util.h"
#include <stdarg.h>
#include <stdio.h> /* vsnprintf */
#include <string.h>

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
#include <openenclave/enclave.h>
#include "enclave/parallel_t.h"
#endif

void *bsearch_ge(const void *key, const void *arr_, size_t num_elems,
        size_t elem_size, int (*comparator)(const void *a, const void *b)) {
    const unsigned char *arr = arr_;
    size_t left = 0;
    size_t right = num_elems;

    while (left < right) {
        size_t mid = (left + right) / 2;
        int cmp = comparator(key, arr + mid * elem_size);
        if (cmp == 0) {
            return (void *) (arr + mid * elem_size);
        } else if (cmp < 0) {
            right = mid;
        } else {
            left = mid + 1;
        }
    }

    return (void *) (arr + left * elem_size);
}

// Returns log2 rounded up.
uint64_t calculatelog2(uint64_t value){
  uint64_t log2v = 0;
  uint64_t temp = 1;
  while(temp<value){
    temp=temp<<1;
    log2v+=1;
  }
  return log2v;
}

// Returns log2 floor.
uint64_t calculatelog2_floor(uint64_t value){
  uint64_t log2v = 0;
  uint64_t temp = 1;
  while(temp<value){
    temp=temp<<1;
    log2v+=1;
  }
  if(temp==value)
    return log2v;
  else
    return log2v-1;
}

// Returns largest power of two less than N
uint64_t pow2_lt(uint64_t N) {
  uint64_t N1 = 1;
  while (N1 < N) {
    N1 <<= 1;
  }
  N1 >>= 1;
  return N1;
}


// Returns largest power of two greater than N
uint64_t pow2_gt(uint64_t N) {
  uint64_t N1 = 1;
  while (N1 < N) {
    N1 <<= 1;
  }
  return N1;
}

int printf(const char* fmt, ...)
{
    char buf[BUFSIZ] = { '\0' };
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(buf, BUFSIZ, fmt, ap);
    va_end(ap);
    ocall_print_string(buf);
    return (int)strnlen(buf, BUFSIZ - 1) + 1;
}