#include "enclave/scalable_oblivious_join.h"
#include <limits.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdio.h>
#include <stdarg.h>
#include <threads.h>
#include <liboblivious/algorithms.h>
#include <liboblivious/primitives.h>
#include "common/elem_t.h"
#include "common/error.h"
#include "common/util.h"
#include "common/ocalls.h"
#include "common/code_conf.h"
#include "enclave/mpi_tls.h"
#include "enclave/parallel_enc.h"
#include "enclave/threading.h"
#include "enclave/oblivious_compact.h"

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
#include <openenclave/enclave.h>
#include "enclave/parallel_t.h"
#endif



static int number_threads;
static bool *control_bit;

int values[] = {
    1004, 1010, 1012, 1017, 1018, 1023, 1027, 1031, 1034, 1035, 1036, 1041,
    1046, 1051, 1062, 1063, 1072, 1075, 1078, 1082, 1083, 1086, 1088, 1096,
    1106, 1116, 1118, 1121, 1122, 1126, 1128, 1140, 1146, 1156, 1160, 1171,
    1172, 1179, 1182, 1187, 1193, 1199, 1207, 1221, 1229, 1239, 1245, 1257,
    1258, 1262, 1271, 1274, 1276, 1283, 1292, 1315, 1323, 1329, 1339, 1344,
    1354, 1360, 1374, 1383, 1419, 1427, 1431, 1432, 1448, 1451, 1454, 1487,
    1496, 1508, 1539, 1566, 1569, 1579, 1587, 1622, 1630, 1647, 1673, 1684,
    1733, 1740, 1742, 1768, 1787, 1789, 1822, 1825, 1888, 1908, 1944, 1969,
    1971, 2036, 2046, 2123, 2152, 2191, 2255, 2307, 2345, 2388, 2411, 2453,
    2468, 2475, 2537, 2705, 2840, 2850, 2963, 3036, 3136, 3411, 3435, 3632,
    3826, 3996, 4333, 4668, 4960, 5738, 6660, 8097, 11289, 80818
};
int random_index;
int random_value;


void reverse(char *s) {
    int i, j;
    char c;

    for (i = 0, j = strlen(s)-1; i<j; i++, j--) {
        c = s[i];
        s[i] = s[j];
        s[j] = c;
    }
}

int my_len(char *data) {
    int i = 0;

    while ((data[i] != '\0') && (i < DATA_LENGTH)) i++;
    
    return i;
}

void itoa(int n, char *s, int *len) {
    int i = 0;
    int sign;

    if ((sign = n) < 0) {
        n = -n;
        i = 0;
    }
        
    do {
        s[i++] = n % 10 + '0';
    } while ((n /= 10) > 0);
    
    if (sign < 0)
        s[i++] = '-';
    s[i] = '\0';
    
    *len = i;
    
    reverse(s);
}

int scalable_oblivious_join_init(int nthreads) {

    number_threads = nthreads;
    return 0;

}

void scalable_oblivious_join_free() {
    return;
}


struct soj_scan_1_args {
    int idx_st;
    int idx_ed;
    bool* control_bit;
    elem_t *arr1;
};


void soj_scan_1(void *voidargs) {
    struct soj_scan_1_args *args = (struct soj_scan_1_args*)voidargs;
    int index_start = args->idx_st;
    int index_end = args->idx_ed;
    bool* cb1 = args->control_bit;
    elem_t *arr1 = args->arr1;

    for (int i = index_start; i < index_end; i++) {
        cb1[i] = (random_value == arr1[i].key);
    }

    return;
}


void scalable_oblivious_join(elem_t *arr, int length1, int length2, char* output_path){
    control_bit = calloc(length1, sizeof(*control_bit));
    (void)length2;
    int length_thread = length1 / number_threads;
    int length_extra = length1 % number_threads;
    struct soj_scan_1_args soj_scan_1_args_[number_threads];
    int index_start_thread[number_threads + 1];
    index_start_thread[0] = 0;
    struct thread_work soj_scan_1_[number_threads - 1];
    int length_result;


    // Use a fixed seed for reproducible results
    // srand();  // Fixed seed (you can choose any integer value)
    srand(time(NULL));  // Use the current time as a seed for randomness
    
    // Pick from your values array
    random_index = rand() % (sizeof(values) / sizeof(values[0]));
    random_value = values[random_index];
    printf("Point Query Search Value: %d\n", random_value);
    init_time2();

    if (number_threads == 1) {
        for (int i = 0; i < length1; i++) {
            control_bit[i] = (88 < arr[i].key);
        }  
    }
    else {
        for (int i = 0; i < number_threads; i++) {
            index_start_thread[i + 1] = index_start_thread[i] + length_thread + (i < length_extra);
        
            soj_scan_1_args_[i].idx_st = index_start_thread[i];
            soj_scan_1_args_[i].idx_ed = index_start_thread[i + 1];
            soj_scan_1_args_[i].control_bit = control_bit;
            soj_scan_1_args_[i].arr1 = arr;

            if (i < number_threads - 1) {
                soj_scan_1_[i].type = THREAD_WORK_SINGLE;
                soj_scan_1_[i].single.func = soj_scan_1;
                soj_scan_1_[i].single.arg = soj_scan_1_args_ + i;
                thread_work_push(soj_scan_1_ + i);
            }
        }
        soj_scan_1(soj_scan_1_args_ + number_threads - 1);
        for (int i = 0; i < number_threads; i++) {
            if (i < number_threads - 1) {
                thread_wait(soj_scan_1_ + i);
            };
        }
    };

    length_result = oblivious_compact_elem(arr, control_bit, length1, 1, number_threads);

    get_time2(true);

    char *char_current = output_path;
    for (int i = 0; i < length_result; i++) {
        int key1 = arr[i].key;

        char string_key1[10];
        int str1_len;
        itoa(key1, string_key1, &str1_len);
        int data_len1 = my_len(arr[i].data);
        
        strncpy(char_current, string_key1, str1_len);
        char_current += str1_len; char_current[0] = ' '; char_current += 1;
        strncpy(char_current, arr[i].data, data_len1);
        char_current += data_len1; char_current[0] = '\n'; char_current += 1;
    }
    char_current[0] = '\0';

    return;
}