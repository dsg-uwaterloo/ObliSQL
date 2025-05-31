#include "enclave/window.h"
#include <assert.h>
#include <errno.h>
#include <limits.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#define WINDOW_INITAL_SIZE 16

static_assert(WINDOW_INITAL_SIZE % CHAR_BIT == 0,
        "WINDOW_INITAL_SIZE must be a multiple of CHAR_BIT");
static_assert(WINDOW_INITAL_SIZE >= CHAR_BIT,
        "WINDOW_INITAL_SIZE must be at least CHAR_BIT");

int window_init(window_t *window) {
    int ret;

    window->window =
        calloc(WINDOW_INITAL_SIZE / CHAR_BIT, sizeof(*window->window));
    if (!window->window) {
        ret = errno;
        goto exit;
    }
    window->window_min = 0;
    window->window_len = WINDOW_INITAL_SIZE;
    window->ring_start = 0;

    ret = 0;

exit:
    return ret;
}

void window_free(window_t *window) {
    free(window->window);
}

int window_add(window_t *restrict window, uint64_t val,
        bool *restrict was_set) {
    int ret;

    /* If the value is less than the window minimum, it was already set. */
    if (val < window->window_min) {
        *was_set = true;
        ret = 0;
        goto exit;
    }

    /* Compute the index of the value we want to set. */
    size_t val_idx = val - window->window_min;

    /* If the index is higher than the highest value we can store in the current
     * window, expand the window. */
    if (val_idx >= window->window_len) {
        unsigned char *new_window =
            realloc(window->window, window->window_len * 2 / CHAR_BIT);
        if (!new_window) {
            ret = errno;
            goto exit;
        }
        window->window = new_window;
        window->window_len *= 2;

        /* Copy the items at the tail of the ring to the new buffer space, then
         * zero it out. */
        memcpy(window->window + window->window_len / 2 / CHAR_BIT,
                window->window, window->ring_start);
        memset(window->window, '\0', window->ring_start);

        /* Zero out the remaining new space. */
        memset(window->window + window->window_len / 2 / CHAR_BIT
                    + window->ring_start,
                '\0',
                (window->window_len / 2 / CHAR_BIT - window->ring_start)
                    * sizeof(*window->window));
    }

    /* Compute the position of the index within the ring. */
    size_t ring_idx =
        (val_idx + window->ring_start * CHAR_BIT) % window->window_len;

    /* Test and set the bit. */
    *was_set =
        (window->window[ring_idx / CHAR_BIT] >> (ring_idx % CHAR_BIT)) & 1;
    window->window[ring_idx / CHAR_BIT] |= 1 << ring_idx % CHAR_BIT;

    /* Advance the sliding window for as long as we have bytes made of all
     * 1s. */
    while (window->window[window->ring_start] == (1u << CHAR_BIT) - 1) {
        window->window[window->ring_start] = 0;
        window->window_min += CHAR_BIT;
        window->ring_start++;
        if (window->ring_start == window->window_len / CHAR_BIT) {
            window->ring_start = 0;
        }
    }

    ret = 0;

exit:
    return ret;
}
