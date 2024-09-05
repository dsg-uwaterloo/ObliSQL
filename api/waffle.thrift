struct sequence_id {
  1: required i64 client_id,
  2: required i64 client_seq_no,
  3: required i64 server_seq_no,
}

service waffle_thrift{
  i64 get_client_id();
  void register_client_id(1: i32 block_id, 2: i64 client_id);
  oneway void async_get(1:sequence_id seq_id, 2:string key);
  oneway void async_put(1:sequence_id seq_id, 2:string key, 3:string value);
  oneway void async_get_batch(1:sequence_id seq_id, 2:list<string> keys);
  oneway void async_put_batch(1:sequence_id seq_id, 2:list<string> keys, 3:list<string> values);
  void init_db(1:list<string> keys, 2:list<string> values);
  void init_args(1: i64 B, 2:i64 R, 3:i64 F, 4:i64 D, 5:i64 C, 6:i64 N);
  string get(1:string key);
  void put(1:string key, 2:string value);
  list<string> get_batch(1:list<string> keys);
  list<string> mix_batch(1:list<string> keys, 2:list<string> values);
  void put_batch(1:list<string> keys, 2:list<string> values);
}

service waffle_thrift_response{
  oneway void async_response(1:sequence_id seq_id, 2:i32 op_code, 3:list<string>result)
}
