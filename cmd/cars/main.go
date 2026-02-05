package main

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

var cars = map[string]string{
	"id1": "Renault Logan",
	"id2": "Renault Duster",
	"id3": "BMW X6",
	"id4": "BMW M5",
	"id5": "VW Passat",
	"id6": "VW Jetta",
	"id7": "Audi A4",
	"id8": "Audi Q7",
}

// carsListFunc — вспомогательная функция для вывода всех машин.
func carsListFunc() []string {
	var list []string
	for _, c := range cars {
		list = append(list, c)
	}
	return list
}

// carFunc — вспомогательная функция для вывода определённой машины.
func carFunc(id string) string {
	if c, ok := cars[id]; ok {
		return c
	}
	return "unknown identifier " + id
}

func carsHandle(rw http.ResponseWriter, r *http.Request) {
	carsList := carsListFunc()
	io.WriteString(rw, strings.Join(carsList, ", "))
}

func carHandle(rw http.ResponseWriter, r *http.Request) {
	carID := r.URL.Query().Get("id")
	if carID == "" {
		http.Error(rw, "carID param is missed", http.StatusBadRequest)
		return
	}
	rw.Write([]byte(carFunc(carID)))
}

func getBrand(brand string) []string {
	var list []string
	for _, c := range cars {
		if strings.HasPrefix(strings.ToLower(c), strings.ToLower(brand)) {
			list = append(list, c)
		}
	}
	return list
}

func getModel(model string) string {
	for _, c := range cars {
		if strings.HasSuffix(strings.ToLower(c), strings.ToLower(model)) {
			return c
		}
	}
	return "unknown model " + model
}

func main() {
	r := chi.NewRouter()
	// определяем хендлер, который выводит все машины
	r.Get("/cars", carsHandle)
	// определяем хендлер, который выводит определённую машину
	r.Get("/car", carHandle)

	r.Route("/cars", func(r chi.Router) {
		r.Get("/", carsHandle)
		r.Route("/{brand}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				brand := chi.URLParam(r, "brand")
				w.Write([]byte(strings.Join(getBrand(brand), ", ")))
			})
			r.Route("/{model}", func(r chi.Router) {
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					model := chi.URLParam(r, "model")
					w.Write([]byte(getModel(model)))
				})
			})
		})
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}
