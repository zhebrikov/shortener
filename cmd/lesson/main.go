package main

import (
	"fmt"
	"log"

	"github.com/monaco-io/request"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func main() {
	var users []User
	c := request.Client{
		URL:    "https://jsonplaceholder.typicode.com/users",
		Method: "GET",
	}
	resp := c.Send().Scan(&users)

	if !resp.OK() {
		// handle error
		log.Println(resp.Error())
	}

	// str := resp.String()
	// bytes := resp.Bytes()

	fmt.Println(users)
}
