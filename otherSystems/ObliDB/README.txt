Link to Repo: https://github.com/SabaEskandarian/ObliDB/

Note: Our changes to the code are present under isv_app/isv_app.cpp 

We added a new benchmarking function called client_server_benchmark. 

The function: 

Runs K number of experiments (specified by the caller, in this case BDB1Index Function)

The Caller function initializes each table (Ranking and Uservisits) 

Within the function, joins represent 5% of the all queries that are run. The rest are divided equally between Point and Range. 
This functionality mimics our implementation and testing. 

In order to introduce the notion of network delay, we add a 5ms delay before the query operation and 5ms after, for a total of 10ms. 
We do this for our system as well. All reported numbers include this delay. 

Query execution code is taken as is from their respective functions. 

------------------------------------
Step 1: Generate Key
------------------------------------
Run the following command to generate an RSA key:
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -pkeyopt rsa_keygen_pubexp:3 -out`

------------------------------------
Step 2: Build App
------------------------------------
Built in Hardware Mode, Release build.


make clean && 
make SGX_DEBUG=0 && 
/opt/intel/sgxsdk/bin/x64/sgx_sign sign -key enclave_private.pem -enclave isv_enclave.so -out isv_enclave.signed.so -config isv_enclave/isv_enclave.config.xml




Files Modified: 
1. isv_app.cpp
2. MakeFile (Updated C++ version being used)


Original Readme:

Prototype source code for ObliDB, an oblivious general-purpose SQL database for the cloud.

------------------------------------
How to Build/Execute the Code
------------------------------------
1. Install Intel(R) SGX SDK for Linux* OS
2. Build the project with the prepared Makefile:
    a. Hardware Mode, Debug build:
        $ make
    b. Hardware Mode, Pre-release build:
        $ make SGX_PRERELEASE=1 SGX_DEBUG=0
    c. Hardware Mode, Release build:
        $ make SGX_DEBUG=0
    d. Simulation Mode, Debug build:
        $ make SGX_MODE=SIM
    e. Simulation Mode, Pre-release build:
        $ make SGX_MODE=SIM SGX_PRERELEASE=1 SGX_DEBUG=0
    f. Simulation Mode, Release build:
        $ make SGX_MODE=SIM SGX_DEBUG=0
3. Execute the binary directly:
    $ ./app
4. Remember to "make clean" before switching build mode
