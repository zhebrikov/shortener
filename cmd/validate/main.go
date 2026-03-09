package main

import (
	"log"
	"reflect"
	"strconv"
	"strings"
)

func Str2Int(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return v
}

func Validate(obj interface{}) bool {
	vobj := reflect.ValueOf(obj)
	objType := vobj.Type() // получаем описание типа

	// перебираем все поля структуры
	for i := 0; i < objType.NumField(); i++ {
		// берём значение текущего поля и проверяем, что это int
		if v, ok := vobj.Field(i).Interface().(int); ok {
			tag, ok := objType.Field(i).Tag.Lookup("limit")
			if !ok || tag == "" {
				continue
			}
			parts := strings.Split(tag, ",")
			lenParts := len(parts)

			if lenParts == 1 {
				if !validateOnlyMin(v, parts[0]) {
					return false
				}
				continue
			}

			if lenParts == 2 {
				if !validateMinMax(v, parts[0], parts[1]) {
					return false
				}
				continue
			}

			return false
		}
	}
	return true
}

func validateOnlyMin(v int, min string) bool {
	minVal := Str2Int(min)
	log.Println(v, minVal, v >= minVal)
	return v >= minVal
}

func validateMinMax(v int, min, max string) bool {
	minVal := Str2Int(min)
	maxVal := Str2Int(max)
	log.Println(v, minVal, maxVal)
	return v >= minVal && v <= maxVal
}
