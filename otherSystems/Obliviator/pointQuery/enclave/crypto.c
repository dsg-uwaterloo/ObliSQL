#include "enclave/crypto.h"
#include <limits.h>
#include <stddef.h>
#include <string.h>
#include <threads.h>
#include <mbedtls/cipher.h>
#include <mbedtls/entropy.h>
#include <mbedtls/gcm.h>
#include "common/error.h"

mbedtls_entropy_context entropy_ctx;

struct thread_local_ctx ctxs[THREAD_LOCAL_LIST_MAXLEN];
size_t ctx_len;
thread_local struct thread_local_ctx *ctx;

const unsigned char zeroes[RAND_BYTES_POOL_LEN];

int rand_init(void) {
    mbedtls_entropy_init(&entropy_ctx);
    return 0;
}

void rand_free(void) {
    for (size_t i = 0; i < ctx_len; i++) {
        mbedtls_cipher_free(&ctxs[i].cipher_ctx);
        ctxs[i].ptr = NULL;
    }
    ctx_len = 0;
    mbedtls_entropy_free(&entropy_ctx);
}

int crypto_ensure_thread_local_ctx_init(void) {
    int ret;

    if (!ctx || !ctx->ptr) {
        size_t idx =
            __atomic_fetch_add(&ctx_len, 1, __ATOMIC_RELAXED);
        if (ctx_len >= THREAD_LOCAL_LIST_MAXLEN) {
            handle_error_string("Too many threads for crypto");
            ret = -1;
            goto exit_dec_ctx_len;
        }
        ctx = &ctxs[idx];
        ctx->ptr = &ctx;

        /* Get seed from entropy. */
        unsigned char seed[16];
        ret = mbedtls_entropy_func(&entropy_ctx, seed, sizeof(seed));
        if (ret) {
            handle_mbedtls_error(ret, "mbedtls_entropy_func");
            goto exit_dec_ctx_len;
        }

        /* Get cipher info. */
        const mbedtls_cipher_info_t *cipherinfo =
            mbedtls_cipher_info_from_type(MBEDTLS_CIPHER_AES_128_CTR);
        if (!cipherinfo) {
            handle_error_string("mbedtls_cipher_info_from_type");
            goto exit_dec_ctx_len;
        }

        /* Setup cipher. */
        mbedtls_cipher_init(&ctx->cipher_ctx);
        ret = mbedtls_cipher_setup(&ctx->cipher_ctx, cipherinfo);
        if (ret) {
            handle_mbedtls_error(ret, "mbedtls_cipher_setup");
            goto exit_free_cipher;
        }
        ret =
            mbedtls_cipher_setkey(&ctx->cipher_ctx, seed, 128, MBEDTLS_ENCRYPT);
        if (ret) {
            handle_mbedtls_error(ret, "mbedtls_cipher_setkey");
            goto exit_free_cipher;
        }

        ctx->rand_bytes_pool_idx = RAND_BYTES_POOL_LEN;
        ctx->rand_bits_left = 0;
    }

    return 0;

exit_free_cipher:
    mbedtls_cipher_free(&ctx->cipher_ctx);
exit_dec_ctx_len:
    __atomic_fetch_sub(&ctx_len, 1, __ATOMIC_RELAXED);
    ctx = NULL;
    return ret;
}

int aad_encrypt(const void *key, const void *plaintext, size_t plaintext_len,
        const void *aad, size_t aad_len, const void *iv, void *ciphertext,
        void *tag) {
    int ret = -1;

    /* Initialize encryption context. */
    mbedtls_gcm_context ctx;
    mbedtls_gcm_init(&ctx);

    /* Initialize key. */
    ret =
        mbedtls_gcm_setkey(&ctx, MBEDTLS_CIPHER_ID_AES, key,
                KEY_LEN * CHAR_BIT);
    if (ret) {
        handle_mbedtls_error(ret, "mbedtls_gcm_setkey");
        goto exit_free_ctx;
    }

    //memcpy(ciphertext, plaintext, plaintext_len);
    //ret = 0;
    //goto exit_free_ctx;

    /* Encrypt. */
    ret = mbedtls_gcm_crypt_and_tag(&ctx, MBEDTLS_GCM_ENCRYPT, plaintext_len,
            iv, IV_LEN, aad, aad_len, plaintext, ciphertext, TAG_LEN, tag);
    if (ret) {
        handle_mbedtls_error(ret, "mbedtls_gcm_crypt_and_tag");
        goto exit_free_ctx;
    }

    ret = 0;

exit_free_ctx:
    mbedtls_gcm_free(&ctx);
    return ret;
}

int aad_decrypt(const void *key, const void *ciphertext, size_t ciphertext_len,
        const void *aad, size_t aad_len, const void *iv, const void *tag,
        void *plaintext) {
    int ret = -1;

    /* Initialize encryption context. */
    mbedtls_gcm_context ctx;
    mbedtls_gcm_init(&ctx);

    /* Initialize key. */
    ret = mbedtls_gcm_setkey(&ctx, MBEDTLS_CIPHER_ID_AES, key, 128);
    if (ret) {
        handle_mbedtls_error(ret, "mbedtls_gcm_setkey");
        goto exit_free_ctx;
    }

    //memcpy(ciphertext, plaintext, plaintext_len);
    //ret = 0;
    //goto exit_free_ctx;

    /* Decrypt. */
    ret = mbedtls_gcm_auth_decrypt(&ctx, ciphertext_len, iv, IV_LEN, aad,
            aad_len, tag, TAG_LEN, ciphertext, plaintext);
    if (ret) {
        handle_mbedtls_error(ret, "mbedtls_gcm_auth_decrypt");
        goto exit_free_ctx;
    }

    ret = 0;

exit_free_ctx:
    mbedtls_gcm_free(&ctx);
    return ret;
}
