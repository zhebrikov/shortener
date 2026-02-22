package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

func main() {
	// data содержит данные в формате gob
	data := []byte{12, 255, 129, 2, 1, 2, 255, 130, 0, 1, 12,
		0, 0, 17, 255, 130, 0, 2, 6, 72, 101, 108, 108,
		111, 44, 5, 119, 111, 114, 108, 100}

	// напишите код, который декодирует data в массив строк
	// 1. создайте буфер `bytes.NewBuffer(data)` для передачи в декодер
	// 2. создайте декодер `dec := gob.NewDecoder(buf)`
	// 3. определите `make([]string, 0)` для получения декодированного слайса
	// 4. декодируйте данные используя функцию `dec.Decode`

	// ...

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var strings []string
	dec.Decode(&strings)
	fmt.Println(strings)
}
