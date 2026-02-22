package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Event struct {
	ID       uint    `json:"id"`
	CarModel string  `json:"car_model"`
	Price    float64 `json:"price"`
}

type Producer struct {
	file    *os.File
	encoder *json.Encoder
}

func NewProducer(filename string) (*Producer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &Producer{file: file, encoder: json.NewEncoder(file)}, nil
}

func (p *Producer) WriteEvent(event *Event) error {
	return p.encoder.Encode(event)
}

func (p *Producer) Close() error {
	return p.file.Close()
}

type Consumer struct {
	file    *os.File
	decoder *json.Decoder
}

func NewConsumer(filename string) (*Consumer, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return &Consumer{file: file, decoder: json.NewDecoder(file)}, nil
}

func (c *Consumer) ReadEvent() (*Event, error) {
	var event Event
	if err := c.decoder.Decode(&event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (c *Consumer) Close() error {
	return c.file.Close()
}

var events = []*Event{
	{
		ID:       1,
		CarModel: "Lada",
		Price:    400000,
	},
	{
		ID:       2,
		CarModel: "Mitsubishi",
		Price:    650000,
	},
	{
		ID:       3,
		CarModel: "Toyota",
		Price:    800000,
	},
	{
		ID:       4,
		CarModel: "BMW",
		Price:    875000,
	},
	{
		ID:       5,
		CarModel: "Mercedes",
		Price:    999999,
	},
}

func main() {
	fileName := "events.log"
	defer os.Remove(fileName)

	Producer, err := NewProducer(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer Producer.Close()

	Consumer, err := NewConsumer(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer Consumer.Close()

	for _, event := range events {
		if err := Producer.WriteEvent(event); err != nil {
			log.Fatal(err)
		}

		readEvent, err := Consumer.ReadEvent()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(readEvent)
	}
}
