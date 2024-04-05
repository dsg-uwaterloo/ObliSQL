# Index 

Branch for Waffle's index structure, in Python (as "pseudocode"). 

The current version of the index is in python_pseudocode/index.py. The index allows a certain number of items per
set and pads with bars ("|") if the set is not complete. An example of output is found below:

```
Index|Students|Age|20|1: pk08|pk04|pk03
Index|Students|Age|20|3: pk06|pk09|pk05
Index|Students|Age|20|2: pk01|pk02|pk07
Index|Students|Age|19|1: pk10||||||||||
```