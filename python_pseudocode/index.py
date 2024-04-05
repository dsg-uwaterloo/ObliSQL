import redis
from math import ceil

# Map for counting:
    # {"table1|age" : {19: 2, 20:1, ...}}
        
class IndexMap:
    def __init__(self):
        self.map = dict()

    def create_outer_key(self, table_name, column_name):
        return f"{table_name}|{column_name}"
    
    def create_inner_key(self, column_value):
        return column_value

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
        
        self.ITEMS_PER_SET = 3
        self.MAX_STRING_LENGTH = 14

    def connect_data_server(self, data_server):
        self.data_server = data_server

    def remove_all_data_from_server(self):
        self.index_server.flushdb()

    # Waffle
    def get(self, key) -> str:
        """
        add read(key) to the batch, returns value
        """
        return self.index_server.get(key)
    
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


    ## tuples are in form (Index|{table_name}|{column_name}|{column_val}|{set_index}, 
    ##                     {concatenated string of pks of set length=ITEMS_PER_SET})
    def create_index(self, table_name, column_name):
        # iterate through all rows and check if it matches table_name|column_name|{pk...}
        for key in self.data_server.scan_iter(f"{table_name}|{column_name}|pk*"):
            # get pk
            pk = key.split("|")[2]
            column_value = self.data_server.get(key)
            # add pk to the index
            self.add_pks(table_name, column_name, column_value, [pk])

    def delete_index(self, table_name, column_name):
        for key in self.index_server.scan_iter(f"Index|{table_name}|{column_name}|*"):
            self.delete(key)

    def add_pks(self, table_name, column_name, column_value, pks):
        """
        Space: O(self.ITEMS_PER_SET)
        
        Let p = len(pks)
        Time: O(p)
              O(p / self.ITEMS_PER_SET) calls to Waffle
        """
        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
    
        pks = set(pks)
        len_pks = len(pks)
        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        set_index = ceil(num_items / self.ITEMS_PER_SET)
        index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)]) 

        mod_num_items = num_items % self.ITEMS_PER_SET

        # fill current set
        if mod_num_items != 0:
            set_string = self.get(index_key).rstrip("|")
            for _ in range(self.ITEMS_PER_SET - mod_num_items):
                if pks:
                    set_string += "|"
                    set_string += pks.pop()
                else:
                    break
            set_string = create_set_string(set_string)
            self.set(index_key, set_string)

        set_index += 1

        # iteratively fill rest
        while pks:
            set_string = ""
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(set_index)]) 
            for _ in range(self.ITEMS_PER_SET):
                if pks:
                    if set_string != "":
                        set_string += "|"
                    set_string += pks.pop()
                else:
                    break
            set_string = create_set_string(set_string)
            self.set(index_key, set_string)
            set_index += 1

        self.index_map.update_feature_count(table_name, column_name, column_value, len_pks)

    def delete_pks(self, table_name, column_name, column_value, pks):
        """
        Two pointers: we have a front set pointer and a back set pointer.
        1. We get the set of pks in the front set and we remove the pks to be deleted.
           Say x pks are removed.
        2. back_set_good is a set of pks that are not to be deleted. Put x pks from this set
           to front_set.
           --> If back_set_good is empty, then move back_set_index up one 
               and replenish until it is non-empty. Then, continue replenishing front set
        
        Space: O(self.ITEMS_PER_SET)
        Time: O(n), where there are n items in the index (for that column_name / column_value)
              O(n / self.ITEMS_PER_SET) calls to Waffle
        """

        def create_set_string(s):
            return s + "|" * (self.MAX_STRING_LENGTH - len(s))
        
        def string_to_set(s):
            cur_set = set(s.split("|"))
            cur_set.discard("")
            return cur_set
        
        pks = set(pks)
        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        
        back_set_index = ceil(num_items / self.ITEMS_PER_SET)
        front_set_index = 1
        back_set_good = set()

        old_back_set_index = back_set_index

        while pks and front_set_index <= back_set_index:
            front_set_index_key = "Index|" + "|".join([table_name, column_name, column_value, str(front_set_index)])
            # pks in the front set before extracting pks to delete
            front_set = string_to_set(self.get(front_set_index_key))
            # remove the pks in front_set that are supposed to be deleted
            front_pks = front_set - pks
            # update remainding pks to be deleted
            pks -= front_set
            # calculate number of pks delete
            front_pks_deleted = len(front_set) - len(front_pks)
            # update count in index_map
            self.index_map.update_feature_count(table_name, column_name, column_value, -front_pks_deleted)

            # while loop to replenish front_pks
            while len(front_pks) < self.ITEMS_PER_SET and front_set_index <= back_set_index:

                # if back_set_good is empty, replenish
                while front_set_index < back_set_index and len(back_set_good) == 0:
                    back_set_index_key = "Index|" + "|".join([table_name, column_name, column_value, str(back_set_index)])
                    back_set = string_to_set(self.get(back_set_index_key))
                    back_pks = back_set - pks
                    pks -= back_set
                    back_pks_deleted = len(back_set) - len(back_pks)
                    self.index_map.update_feature_count(table_name, column_name, column_value, -back_pks_deleted)

                    back_set_index -= 1
                    if back_pks:
                        back_set_good = back_pks
                    
                while back_set_good and len(front_pks) < self.ITEMS_PER_SET:
                    front_pks.add(back_set_good.pop())  

                if front_set_index == back_set_index:
                    break

            # add back front set
            if front_pks:
                self.set(front_set_index_key, create_set_string("|".join(list(front_pks))))

            front_set_index += 1

        if back_set_good:
            last_set_index_key = "Index|" + "|".join([table_name, column_name, column_value, str(back_set_index+1)])
            self.set(last_set_index_key, create_set_string("|".join(list(back_set_good))))

        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        new_back_set_index = ceil(num_items / self.ITEMS_PER_SET)

        for i in range(new_back_set_index+1, old_back_set_index+1):
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(i)])
            self.delete(index_key)

    def fetch_values(self, table_name, column_name, column_value):
        num_items = self.index_map.get_value_count(table_name, column_name, column_value)
        set_index = ceil(num_items / self.ITEMS_PER_SET)
        for i in range(1, set_index+1):
            index_key = "Index|" + "|".join([table_name, column_name, column_value, str(i)])
            set_string = self.get(index_key).rstrip("|")
            pks = set_string.split("|")
            for pk in pks:
                yield pk
                
    # debugging
    def get_data_from_server(self, table_name, column_name):
        for key in self.index_server.scan_iter(f"Index|{table_name}|{column_name}|*|*"):
            val = self.index_server.get(key)
            print(key + ": " + val)


## create fake data
## table in the form (Name, Age, Major)
    ## (Bob, 20, Pure Math)
    ## (Alice, 19, Physics) ...
table_name = "Students"
column_names = ["Name","Age", "Major"]
names = ["Alice", "Bob", "Charlie", "Dave", "Eve", "Frank", "Grace", "Heidi", "Ivan", "Joline"]
ages = ["20", "20", "20", "20", "20", "20", "20", "20", "20", "19"]
majors = ["Pure Mathematics"] * 5 + ["Physics"] * 4 + ["Data Science"]
rows = [(names[i], ages[i], majors[i]) for i in range(len(names))]

## put data in server
data_server = DataServer()
data_server.add_data_to_server(rows, table_name, column_names)
data_server.get_data_from_server(table_name, column_names)


def test_index():
    index_server = Index(data_server=data_server)
    # index server
    index_server.create_index(table_name, "Age")
    index_server.get_data_from_server(table_name, "Age")
    index_server.delete_pks(table_name, "Age", "20", ["pk02", "pk03"])
    print("***********************************************")
    index_server.get_data_from_server(table_name, "Age")
    index_server.add_pks(table_name, "Age", "20", ["pk21", "pk22", "pk23", "pk24", "pk25", "pk26"])
    print("***********************************************")
    index_server.get_data_from_server(table_name, "Age")
    print("***********************************************")
    generator = index_server.fetch_values(table_name, "Age", "20")
    print([i for i in generator])
    index_server.remove_all_data_from_server()

test_index()