package main

import (
	"testing"
)

type User struct {
	Nick string
	Age  int `limit:"18"`
	Rate int `limit:"0,100"`
}

func TestValidate(t *testing.T) {
	var table = []struct {
		name string
		age  int
		rate int
		want bool
	}{
		{"admin", 20, 88, true},
		{"su", 45, 10, true},
		{"", 16, 5, false},
		{"usr", 24, -2, false},
		{"john", 18, 0, true},
		{"usr2", 30, 200, false},
	}
	for _, v := range table {
		if Validate(User{v.name, v.age, v.rate}) != v.want {
			t.Errorf("Не прошла проверку запись %s", v.name)
		}
	}
}
