Repo link for their code: https://zenodo.org/records/15169905


PointQuery and RangeQuery are copies of their "operator_1" folder. We modified <folder>/enclave/scalable_oblivious_join.c to match our queries implementation. Script used to run experiments is under myExperiments. We used the program output values for execution time.

For Joins, we used their implementation as is, using the sample_inputs provided under myExperiments. For joins, the execution time is the sum of all 3 parts returned by the join program. (operator 3_1, 3_2, 3_3). 

We used the provided parallel.conf file for 32 threads.