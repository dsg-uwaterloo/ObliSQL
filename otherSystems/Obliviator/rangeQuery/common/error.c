#include "error.h"
#include <stdarg.h>
#include <stdio.h>
#include <mbedtls/error.h>
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
#include <openenclave/bits/result.h>
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */

void _handle_mbedtls_error(int ret, const char *msg, const char *file,
        int line) {
    char error[256];
    mbedtls_strerror(ret, error, sizeof(error));
    fprintf(stderr, "%s:%d: %s: %s\n", file, line, msg, error);
}

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
void _handle_oe_error(oe_result_t result, const char *msg, const char *file,
        int line) {
    fprintf(stderr, "%s:%d: %s: %s\n", file, line, msg, oe_result_str(result));
}
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */

void _handle_error_string(const char *file, int line, const char *format, ...) {
    va_list args;
    va_start(args, format);
    fprintf(stderr, "%s:%d: ", file, line);
    vfprintf(stderr, format, args);
    fprintf(stderr, "\n");
}
