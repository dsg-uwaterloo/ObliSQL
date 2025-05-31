#include "host/error.h"
#include <stdio.h>
#include <mpi.h>

void _handle_mpi_error(int errno, const char *msg, const char *file, int line) {
    char error[256];
    int len = 256;
    MPI_Error_string(errno, error, &len);
    fprintf(stderr, "%s:%d: %s: %s\n", file, line, msg, error);
}
