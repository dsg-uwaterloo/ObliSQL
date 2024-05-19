
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