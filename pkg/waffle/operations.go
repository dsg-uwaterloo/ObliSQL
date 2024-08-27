package main

type Operation struct {
	Key    string
	Value  string
	Result chan string
}

func NewOperation() *Operation {
	return &Operation{}
}

func (o *Operation) Equals(other *Operation) bool {
	return o.Key == other.Key && o.Value == other.Value
}

func (o *Operation) NotEquals(other *Operation) bool {
	return !o.Equals(other)
}
