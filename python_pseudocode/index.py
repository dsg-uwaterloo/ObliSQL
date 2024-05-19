import redis
from math import ceil
import time
import copy

# Map for counting:
    # {"table1|age" : {19: 2, 20:1, ...}}
# indexInformation in the pseudcode document       
class IndexMap:
    def __init__(self):
        self.map = dict()

    def get(self, s):
        if s in self.map:
            return self.map[s]
        else:
            return None

    def create_outer_key(self, table_name, column_name):
        return f"{table_name}|{column_name}"
    
    def create_inner_key(self, column_value):
        return column_value
    
    def delete_on_column(self, table_name, column_name):
        outer_key = self.create_outer_key(table_name, column_name)
        if outer_key in self.map:
            del self.map[outer_key]

    def update_feature_count(self, table_name, column_name, column_value, change):
        # create entry if necessary
        outer_key = self.create_outer_key(table_name, column_name)
        if outer_key not in self.map:
            self.map[outer_key] = dict()

        # create entry within map if necessary
        inner_key = self.create_inner_key(column_value)
        if inner_key not in self.map[outer_key]:
            self.map[outer_key][inner_key] = 0

        # add change to current count
        new_value = self.map[outer_key][inner_key] + change
        if new_value <= 0:
            del self.map[outer_key][inner_key]
        else:
            self.map[outer_key][inner_key] = new_value

        return max(0, new_value)
    
    def get_value_count(self, table_name, column_name, column_value):
        outer_key = self.create_outer_key(table_name, column_name)
        inner_key = self.create_inner_key(column_value)

        if outer_key in self.map and inner_key in self.map[outer_key]:
            return self.map[outer_key][inner_key]
        return 0
    
    def get_total_sum(self, table_name, column_name):
        outer_key = self.create_outer_key(table_name, column_name)
        if outer_key in self.map:
            final_sum = 0
            for column_value, value_count in self.map[outer_key]:
                final_sum += (column_value * value_count)
            return final_sum
        # no index on table_name|column_name
        return None
    
    def get_total_count(self, table_name, column_name):
        outer_key = self.create_outer_key(table_name, column_name)
        if outer_key in self.map:
            final_count = 0
            for column_value, value_count in self.map[outer_key]:
                final_sum += value_count
            return final_count
        # no index on table_name|column_name
        return None
    
    def get_total_avg(self, table_name, column_name):
        outer_key = self.create_outer_key(table_name, column_name)
        if outer_key in self.map:
            final_sum = 0
            final_count = 0
            for column_value, value_count in self.map[outer_key]:
                final_sum += (column_value * value_count)
                final_count += value_count
            return final_sum / final_count
        # no index on table_name|column_name
        return None
    

class DataServer:
    def __init__(self):
        self.server = redis.Redis(host='localhost', port=6379, decode_responses=True)
        self.counter = 0

    def scan_iter(self, pattern):
        return self.server.scan_iter(pattern)
    
    # Waffle
    def get(self, key):
        """
        add read(key) to the batch, returns value
        """
        return self.server.get(key)
    
    # Waffle
    def set(self, key, value):
        """
        add write(key, val) to the batch
        """
        return self.server.set(key, value)

    # where data is a list of tuples with PK as the first column
    # column_names does not include PK's
    def add_data_to_server(self, data, table_name, column_names):
        ## put fake info as ({table_name}|{column_name}|{PK}, {value}) into data server
        for row in data:
            self.counter += 1
            pk = "pk" + "{:02d}".format(self.counter)
            for i, val in enumerate(row):
                key = "|".join([table_name, column_names[i], pk])
                self.set(key, val)

    # debugging
    def get_data_from_server(self, table_name, column_names):
        for i in range(self.counter):
            for column_name in column_names:
                key = "|".join([table_name, column_name, "pk" + "{:02d}".format(i)])
                val = self.get(key)
                if val: # if val is not None -> key exists in index server
                    print(key + ": " + val)

    # warning: if index server is on the same, it will wipe out index info too
    def remove_all_data_from_server(self):
        self.server.flushdb()


class Index:
    def __init__(self, data_server):
        self.data_server = data_server
        self.index_server = redis.Redis(host='localhost', port=6379, decode_responses=True)
        self.index_map = IndexMap()
        
        self.ITEMS_PER_SET = 4
        self.MAX_STRING_LENGTH = (4*4)+(4-1)

        self.MAX_NUM_CURRENT_SETS = 2
        self.request_read_batch = set()

    def connect_data_server(self, data_server):
        self.data_server = data_server

    def remove_all_data_from_server(self):
        self.index_server.flushdb()

    # Waffle
    def get(self, key) -> str:
        """
        add read(key) to the batch, returns value
        does not delete key when read
        """
        return self.index_server.get(key)
    
    def get_delete(self, key) -> str:
        """
        add read(key) to the batch, returns value
        does not delete key when read
        """
        ans = self.index_server.get(key)
        self.index_server.delete(key)
        return ans
    
    def set(self, key, val):
        """
        add write(key, val) to the batch
        """
        self.index_server.set(key, val)

    def delete(self, key):
        """
        add delete(key) to the batch
        """
        self.index_server.delete(key)

    def get_read_batch(self):
        """
        get values for each key in the self.request_read_batch, then empties it
        Hypothesis: sending read request in batches is quicker than individually sending
        """

        ans = dict([(key, self.get(key)) for key in self.request_read_batch])
        for key in self.request_read_batch:
            self.delete(key)
        self.request_read_batch = set()
        return ans


    ## tuples are in form (Index|{table_name}|{column_name}|{column_val}|{set_index}, 
    ##                     {concatenated string of pks of set length=ITEMS_PER_SET})
    def create_index(self, table_name, column_name, batch_size=1):
        # iterate through all rows and check if it matches table_name|column_name|{pk...}

        col_vals_to_pks = dict()
        num_in_batch = 0
        # batch add_pks responses in batches of x (random number for now)
        for key in self.data_server.scan_iter(f"{table_name}|{column_name}|*"):
            # get pk
            pk = key.split("|")[2]
            column_value = self.data_server.get(key)

            # add to batch
            if column_value not in col_vals_to_pks:
                col_vals_to_pks[column_value] = set()
            col_vals_to_pks[column_value].add(pk)
            num_in_batch += 1

            # if batch_num = x, add pks
            if num_in_batch == batch_size:
                for column_value, pks in col_vals_to_pks.items():
                    self.add_pks(table_name, column_name, column_value, pks)
                col_vals_to_pks = dict()
                num_in_batch = 0

        # exited for loop, deal with rest
        for column_value, pks in col_vals_to_pks.items():
            self.add_pks(table_name, column_name, column_value, pks)


    def delete_index(self, table_name, column_name):
        for key in self.index_server.scan_iter(f"Index|{table_name}|{column_name}|*"):
            self.delete(key)
        self.index_map.delete_on_column(table_name, column_name)


    def add_pks(self, table_name, column_name, column_value, pks):
        """
        Space: O(self.ITEMS_PER_SET)
        
        Let p = len(pks)
        Time: O(p)
              O(p / self.ITEMS_PER_SET) calls to Waffle
        """
        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
    
        def split_set(s):
            return set(s.rstrip("|").split("|"))
        
        pks = set(pks)

        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        set_index = ceil(num_items / self.ITEMS_PER_SET)

        # filter for duplicates
        for i in range(1, set_index+1):
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(i)]) 
            pks_set = split_set(self.get(index_key))
            pks = pks.difference(pks_set)
        
        len_pks = len(pks) 
        mod_num_items = num_items % self.ITEMS_PER_SET

        # init set_string
        if mod_num_items != 0:
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)]) 
            set_string = split_set(self.get_delete(index_key))
        else:
            set_index += 1
            set_string = set()

        # iteratively fill rest
        while pks:
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)]) 
            while len(set_string) < self.ITEMS_PER_SET and pks:
                set_string.add(pks.pop())
            self.set(index_key, create_set_string("|".join(set_string)))
            set_index += 1
            set_string = set()

        self.index_map.update_feature_count(table_name, column_name, column_value, len_pks)


    def delete_values(self, table_name, column_name, cv_to_pks):
        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
        
        def split_set(s):
            return set(s.rstrip("|").split("|"))
        
        for column_value, pks_to_delete in cv_to_pks:
            num_items = self.index_map.get_value_count(table_name, column_name, column_value)
            total_set_index = ceil(num_items / self.ITEMS_PER_SET)

            good_pks = set()
            good_index = 1
            remaining_pks_to_delete = pks_to_delete
            for set_index in range(1, total_set_index+1):
                index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)]) 
                current_pks = split_set(self.get_delete(index_key))

                remove_pks = current_pks.intersection(remaining_pks_to_delete)
                current_pks = current_pks.difference(remove_pks)
                remaining_pks_to_delete = remaining_pks_to_delete.difference(remove_pks)
                good_pks = good_pks.union(current_pks)

                if len(good_pks) >= self.ITEMS_PER_SET:
                    good_index_key = "Index|" + "|".join([table_name, column_name, column_value, str(good_index)])
                    new_pks = set()
                    for _ in range(self.ITEMS_PER_SET):
                        new_pks.add(good_pks.pop())
                    self.set(good_index_key, create_set_string("|".join(new_pks)))
                    good_index += 1

            if len(good_pks) >= 1:
                good_index_key = "Index|" + "|".join([table_name, column_name, column_value, str(good_index)])
                self.set(good_index_key, create_set_string("|".join(good_pks)))

            self.index_map.update_feature_count(table_name, column_name, column_value, len(remaining_pks_to_delete) - len(pks_to_delete))


    def delete_values_v2(self, table_name, column_name, cv_to_pks: dict):
        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
        
        def split_set(s):
            return set(s.rstrip("|").split("|"))
        
        good_indexes = dict([(k,1) for k in cv_to_pks.keys()])
        good_leftovers = dict([(k,set()) for k in cv_to_pks.keys()]) 
        num_of_current_sets = 0
        upload_last = set() # set of pks that are finished already

        for column_value in cv_to_pks:
            num_items = self.index_map.get_value_count(table_name, column_name, column_value)
            total_set_index = ceil(num_items / self.ITEMS_PER_SET)

            for set_index in range(1, total_set_index+1):

                if num_of_current_sets < self.MAX_NUM_CURRENT_SETS:
                    index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)])
                    self.request_read_batch.add(index_key)
                    num_of_current_sets += 1

                if set_index == total_set_index:
                    upload_last.add(column_value)

                if num_of_current_sets == self.MAX_NUM_CURRENT_SETS:
                    res = self.get_read_batch()
                    num_of_current_sets = 0

                    batch_col_vals = dict()
                    for res_key, res_v in [(k.split("|")[-2], split_set(v)) for k, v in res.items()]:
                        batch_col_vals[res_key] = batch_col_vals.get(res_key, set()).union(res_v)

                    for batch_col_val, batch_pks in batch_col_vals.items():

                        num_pks_deleted = len(batch_pks & cv_to_pks[batch_col_val])
                        large_set = batch_pks.difference(cv_to_pks[batch_col_val])
                        large_set = large_set.union(good_leftovers[batch_col_val])

                        good_leftovers[batch_col_val] = set()

                        new_pks = set()
                        while len(large_set) >= self.ITEMS_PER_SET:
                            for _ in range(self.ITEMS_PER_SET):
                                new_pks.add(large_set.pop())
                            
                            index_key = "Index|" + "|".join([table_name, column_name, batch_col_val, str(good_indexes[batch_col_val])])
                            good_indexes[batch_col_val] += 1
                            self.set(index_key, create_set_string("|".join(new_pks)))
                            new_pks = set()
    
                        if batch_col_val in upload_last and large_set:
                            index_key = "Index|" + "|".join([table_name, column_name, batch_col_val, str(good_indexes[batch_col_val])])
                            good_indexes[batch_col_val] += 1
                            self.set(index_key, create_set_string("|".join(large_set)))
                        elif large_set:
                            good_leftovers[batch_col_val] = large_set

                        self.index_map.update_feature_count(table_name, column_name, column_value, -num_pks_deleted)
            
        if num_of_current_sets != 0:
            res = self.get_read_batch()
            num_of_current_sets = 0

            batch_col_vals = dict()
            for res_key, res_v in [(k.split("|")[-2], split_set(v)) for k, v in res.items()]:
                batch_col_vals[res_key] = batch_col_vals.get(res_key, set()).union(res_v)
            
            for batch_col_val, batch_pks in batch_col_vals.items():
                num_pks_deleted = len(batch_pks.intersection(cv_to_pks[batch_col_val]))
                large_set = batch_pks.difference(cv_to_pks[batch_col_val])
                large_set = large_set.union(good_leftovers[batch_col_val])
                good_leftovers[batch_col_val] = set()
                
                new_pks = set()

                while large_set:
                    for _ in range(self.ITEMS_PER_SET):
                        new_pks.add(large_set.pop())
                        if not large_set:
                            break
                    
                    index_key = "Index|" + "|".join([table_name, column_name, batch_col_val, str(good_indexes[batch_col_val])])
                    good_indexes[batch_col_val] += 1
                    self.set(index_key, create_set_string("|".join(new_pks)))
                    new_pks = set()
                
                self.index_map.update_feature_count(table_name, column_name, column_value, -num_pks_deleted)
            

    def full_update_on_col(self, table_name, column_name, new_value):
        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
        
        def split_set(s):
            return set(s.rstrip("|").split("|"))
        
        new_set_index = 1
        pks_leftover = set()

        if new_value in self.index_map.get(f"{table_name}|{column_name}"):
            num_items = self.index_map.get_value_count(table_name, column_name, new_value)
            set_index = ceil(num_items / self.ITEMS_PER_SET)

            if num_items % self.ITEMS_PER_SET == 0:
                new_set_index = set_index + 1
            else:
                new_set_index = set_index
                index_key = "Index|" + "|".join([table_name, column_name, new_value, str(set_index)])
                pks_leftover = split_set(self.get_delete(index_key))

        col_val_num_items = copy.deepcopy(self.index_map.get(f"{table_name}|{column_name}"))
        for col_val, num_items in col_val_num_items.items():
            if col_val == new_value:
                continue

            num_items = self.index_map.get_value_count(table_name, column_name, col_val)
            set_index = ceil(num_items / self.ITEMS_PER_SET) 
            
            for i in range(1, set_index+1):
                index_key = "Index|" + "|".join([table_name, column_name, col_val, str(i)])
                pks_set = split_set(self.get_delete(index_key))
                if i < set_index or num_items % self.ITEMS_PER_SET == 0:
                    new_key_index = "Index|" + "|".join([table_name, column_name, new_value, str(new_set_index)])
                    self.set(new_key_index, create_set_string("|".join(pks_set)))
                    new_set_index += 1
                else:
                    pks_leftover = pks_leftover.union(pks_set)
            
            if len(pks_leftover) >= self.ITEMS_PER_SET:
                new_pks = set()
                for _ in range(self.ITEMS_PER_SET):
                    new_pks.add(pks_leftover.pop())
                new_key_index = "Index|" + "|".join([table_name, column_name, new_value, str(new_set_index)])
                self.set(new_key_index, create_set_string("|".join(new_pks)))
                new_set_index += 1
            
            self.index_map.update_feature_count(table_name, column_name, col_val, -num_items)
            self.index_map.update_feature_count(table_name, column_name, new_value, num_items)

        if pks_leftover:
            new_key_index = "Index|" + "|".join([table_name, column_name, new_value, str(new_set_index)])
            self.set(new_key_index, create_set_string("|".join(pks_leftover)))


    def fetch_values(self, table_name, column_name, column_value):
        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        set_index = ceil(num_items / self.ITEMS_PER_SET)
        for i in range(1, set_index+1):
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(i)])
            pks = self.get(index_key).rstrip("|").split("|")
            for pk in pks:
                yield pk
                
    # debugging
    def get_data_from_server(self, table_name, column_name):
        for key in self.index_server.scan_iter(f"Index|{table_name}|{column_name}|*|*"):
            val = self.index_server.get(key)
            print(key + ": " + val)


# create fake data
# table in the form (Name, Age, Major)
    # (Bob, 20, Pure Math)
    # (Alice, 19, Physics) ...
table_name = "Students"
column_names = ["Name","Age", "Major"]
names = ["Alice", "Bob", "Charlie", "Dave", "Eve", "Frank", "Grace", "Heidi", "Ivan", "Joline"]
ages = ["20", "20", "20", "20", "20", "20", "20", "20", "20", "19"]
majors = ["Pure Mathematics"] * 5 + ["Physics"] * 4 + ["Data Science"]
rows = [(names[i], ages[i], majors[i]) for i in range(len(names))]

## put data in server
data_server = DataServer()
data_server.remove_all_data_from_server()
data_server.add_data_to_server(rows, table_name, column_names)
data_server.get_data_from_server(table_name, column_names)


def test_index_correctness():
    index_server = Index(data_server=data_server)
    # index server
    index_server.create_index(table_name, "Age")
    index_server.get_data_from_server(table_name, "Age")
    index_server.delete_values_v2(table_name, "Age", {"20" : set(["pk08", "pk09"])})
    print("***********************************************")
    index_server.get_data_from_server(table_name, "Age")
    index_server.add_pks(table_name, "Age", "20", ["pk21", "pk22", "pk23", "pk24", "pk25", "pk26"])
    print("***********************************************")
    index_server.get_data_from_server(table_name, "Age")
    index_server.full_update_on_col(table_name, "Age", "19")
    print("***********************************************")
    index_server.get_data_from_server(table_name, "Age")
    index_server.remove_all_data_from_server()

test_index_correctness()


#################################################### other test (for speed) ###############################################
data_server = DataServer()

def load_data_into_data_server():
    data_server.remove_all_data_from_server()

    with open("serverInput.txt", "r") as fhand:
        count = 0
        for line in fhand:
            line = line.rstrip().split(" ")
            key = line[0].replace("/", "|")
            val = line[1]

            if "index" not in key:
                data_server.set(key, val)
            count += 1
            if count % 10000 == 0:
                print(count)


index_server = Index(data_server=data_server)

table_name = "customer"
column_name = "c_state"

def create_index_on_customer(batch_size, n=3):

    times = list()
    for _ in range(n):
        index_server.delete_index(table_name, column_name)
        start_time = time.time()
        index_server.create_index(table_name, column_name, batch_size)
        times.append(time.time() - start_time)
    print(f"{batch_size}: {sum(times) / n}")


def test_index_delete():
    print(index_server.get("Index|customer|c_state|AJ|1"))
    print(index_server.get("Index|customer|c_state|AJ|2"))
    index_server.delete_values_v2(table_name, column_name, {"AJ" : set(["1390","17752","20866","21439"])})
    print(index_server.get("Index|customer|c_state|AJ|1"))
    print(index_server.get("Index|customer|c_state|AJ|2"))

    # 17752|29696|1390|23419|6882|22497|16810|7931|21439|11885|||
    # 15218|20866|4955|27262|||||||||||||||||||||||||||||||||||||
    # 6882|7931|16810|11885|4955|22497|27262|15218|23419|29696|||
    # None

def test_create_index_batch():
    create_index_on_customer(1)
    create_index_on_customer(10)
    create_index_on_customer(50)
    create_index_on_customer(100)
    create_index_on_customer(500)
    create_index_on_customer(1000)
    create_index_on_customer(5000)

    # For n=3: strong relationship between batch size and seconds taken to create
    # 1: 62.0456870396932
    # 10: 65.97338342666626
    # 50: 60.59136907259623
    # 100: 55.113728523254395
    # 500: 38.47518666585287
    # 1000: 54.44247055053711
    # 5000: 27.383632977803547

# test_index_delete()
#test_create_index_batch()

