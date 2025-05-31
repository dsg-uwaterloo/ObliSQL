#ifndef DISTRIBUTED_SGX_SORT_COMMON_CRYPTO_H
#define DISTRIBUTED_SGX_SORT_COMMON_CRYPTO_H

#include <limits.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <threads.h>
#include <mbedtls/cipher.h>
#include <mbedtls/entropy.h>
#include "common/defs.h"
#include "common/error.h"

#define KEY_LEN 16
#define IV_LEN 12
#define TAG_LEN 16

#define THREAD_LOCAL_LIST_MAXLEN 64
#define RAND_BYTES_POOL_LEN 1048576

extern mbedtls_entropy_context entropy_ctx;

struct thread_local_ctx {
    mbedtls_cipher_context_t cipher_ctx;
    unsigned char rand_counter[16];
    unsigned char rand_bytes_pool[RAND_BYTES_POOL_LEN];
    size_t rand_bytes_pool_idx;
    unsigned long rand_bits;
    size_t rand_bits_left;
    struct thread_local_ctx **ptr;
};

extern thread_local struct thread_local_ctx *ctx;

int crypto_ensure_thread_local_ctx_init(void);

int rand_init(void);
void rand_free(void);

extern const unsigned char zeroes[RAND_BYTES_POOL_LEN];

static inline int rand_get_random_bytes(void *buf_, size_t n) {
    unsigned char *buf = buf_;
    int ret;

    if (n > sizeof(zeroes)) {
        ret = -1;
        goto exit;
    }

    ret = crypto_ensure_thread_local_ctx_init();
    if (ret) {
        goto exit;
    }

    size_t olen;
    ret = mbedtls_cipher_update(&ctx->cipher_ctx, zeroes, n, buf, &olen);
    if (ret) {
        handle_mbedtls_error(ret, "mbedtls_cipher_crypt");
        goto exit;
    }
    for (size_t i = 0; i < sizeof(ctx->rand_counter); i++) {
        ctx->rand_counter[i]++;
        if (ctx->rand_counter[i] != 0) {
            break;
        }
    }

    ret = 0;

exit:
    return ret;
}

static inline int rand_read(void *buf_, size_t n) {
    unsigned char *buf = buf_;
    int ret;

    ret = crypto_ensure_thread_local_ctx_init();
    if (ret) {
        goto exit;
    }

    /* For multiples of RAND_BYTES_POOL_LEN, get random bytes and put them
     * directly in the buffer, bypassing the pool. */
    if (n >= RAND_BYTES_POOL_LEN) {
        size_t bytes_to_get = n - n % RAND_BYTES_POOL_LEN;
        ret = rand_get_random_bytes(buf, bytes_to_get);
        if (ret) {
            handle_error_string("Error getting new random bytes");
            goto exit;
        }
        buf += bytes_to_get;
        n -= bytes_to_get;
    }

    /* For remaining bytes < RAND_BYTES_POOL_LEN, copy any bytes we have
     * remaining in the pool. */
    size_t bytes_to_get =
        MIN(n, RAND_BYTES_POOL_LEN - ctx->rand_bytes_pool_idx);
    memcpy(buf, ctx->rand_bytes_pool + ctx->rand_bytes_pool_idx, bytes_to_get);
    buf += bytes_to_get;
    n -= bytes_to_get;
    ctx->rand_bytes_pool_idx += bytes_to_get;

    /* If there are still bytes left, replenish the pool and copy the remainder.
     * This should only be the case once since n < RAND_BYTES_POOL_LEN. */
    if (n) {
        ret =
            rand_get_random_bytes(ctx->rand_bytes_pool,
                    sizeof(ctx->rand_bytes_pool));
        if (ret) {
            handle_error_string("Error getting new random bytes");
            goto exit;
        }
        ctx->rand_bytes_pool_idx = 0;

        memcpy(buf, ctx->rand_bytes_pool, n);
        buf += n;
        n -= n;
        ctx->rand_bytes_pool_idx += n;
    }

exit:
    return ret;
}

static inline int rand_bit(bool *bit) {
    int ret;

    ret = crypto_ensure_thread_local_ctx_init();
    if (ret) {
        goto exit;
    }

    if (ctx->rand_bits_left == 0) {
        ret = rand_read(&ctx->rand_bits, sizeof(ctx->rand_bits));
        if (ret) {
            goto exit;
        }
        ctx->rand_bits_left = sizeof(ctx->rand_bits) * CHAR_BIT;
    }

    *bit = ctx->rand_bits & 1;
    ctx->rand_bits >>= 1;

exit:
    return ret;
}

int aad_encrypt(const void *key, const void *plaintext, size_t plaintext_len,
        const void *aad, size_t aad_len, const void *iv, void *ciphertext,
        void *tag);
int aad_decrypt(const void *key, const void *ciphertext, size_t ciphertext_len,
        const void *aad, size_t aad_len, const void *iv, const void *tag,
        void *plaintext);

#endif /* distributed-sgx-sort/common/crypto.h */
