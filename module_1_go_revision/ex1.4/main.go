package main

import "fmt"

/*
Define an interface Storer with two methods:
Save(id string, data []byte) error and Load(id string) ([]byte, error).
Write two structs that implement it:
MemoryStore (stores in a map[string][]byte) and
NullStore (does nothing, always returns nil). Write a function
processData(s Storer, id string, data []byte) that saves then loads.
*/

type Storer interface {
	Save(id string, data []byte) error
	Load(id string) ([]byte, error)
}

type MemoryStore struct {
	hashmap map[string][]byte
}

type NullStore struct {
}

func (ms *MemoryStore) Save(id string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("Error: There is no data to be saved\n")
	}

	ms.hashmap[id] = data

	return nil
}

func (ms *MemoryStore) Load(id string) ([]byte, error) {

	if id == "" {
		return nil, fmt.Errorf("Error: id is none\n")
	}

	data := ms.hashmap[id]

	return data, nil

}

func (ns *NullStore) Save(id string, data []byte) error {
	return nil // does nothing
}

func (ns *NullStore) Load(id string) ([]byte, error) {
	return nil, nil // does nothing
}

func ProcessData(s Storer, id string, data []byte) {

	err := s.Save(id, data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	data, err = s.Load(id)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Printf("Loaded %s\n", data)
}

func main() {
	ms := MemoryStore{
		hashmap: make(map[string][]byte),
	}

	ProcessData(&ms, "1", []byte("Hi there"))

}
