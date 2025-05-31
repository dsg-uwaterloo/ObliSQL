#ifndef DISTIRUBTED_SGX_SORT_ENCLAVE_WINDOW_H
#define DISTIRUBTED_SGX_SORT_ENCLAVE_WINDOW_H

#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

typedef struct window {
    unsigned char *restrict window;
    uint64_t window_min;
    size_t window_len;
    size_t ring_start;
} window_t;

int window_init(window_t *window);
void window_free(window_t *window);

int window_add(window_t *window, uint64_t val, bool *was_set);

#endif /* DISTIRUBTED_SGX_SORT_ENCLAVE_WINDOW_H */
