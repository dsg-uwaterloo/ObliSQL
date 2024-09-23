#!/bin/bash

OBLIVIOUS_PROXY_HOST=""
BATCH_HANDLER_HOST=""
RESOLVER_HOST=""
CLIENT_HOST=""

PROXY_TYPE="Waffle" #Waffle or ORAM

# Define array of b values
b=(1200)

# Define array of cache values
C=(2)

# Define array of n (num of cores) values
n=(1)

# Define array of f_D values
F=(100)

# Define array of D values
D=(100000)

# Define array of R values
R=(800)

# Define array of number of executors (num)

num=(1)

touch 