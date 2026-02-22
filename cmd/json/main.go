package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type Data struct {
	ID      int    `json:"-"`
	Name    string `json:"name"`
	Company string `json:"comp,omitempty"`
}

func main() {
	foo := []Data{
		{
			ID:   10,
			Name: "Gopher",
		},
		{
			Name:    "Вася",
			Company: "Яндекс",
		},
	}
	out, err := json.Marshal(foo)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}
