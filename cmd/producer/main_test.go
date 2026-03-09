package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProducer(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")

	producer, err := NewProducer(filename)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}
	defer producer.Close()

	if producer.file == nil {
		t.Error("producer.file is nil")
	}
	if producer.encoder == nil {
		t.Error("producer.encoder is nil")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func TestNewProducer_InvalidPath(t *testing.T) {
	// путь к несуществующей директории
	filename := filepath.Join(t.TempDir(), "nonexistent", "sub", "events.log")

	_, err := NewProducer(filename)
	if err == nil {
		t.Error("NewProducer() expected error for invalid path")
	}
}

func TestProducer_WriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")

	producer, err := NewProducer(filename)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}
	defer producer.Close()

	event := &Event{ID: 1, CarModel: "Lada", Price: 400000}
	if err := producer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}
}

func TestProducer_Close(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")

	producer, err := NewProducer(filename)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}

	if err := producer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// повторный Close не должен паниковать (os.File допускает повторный Close)
	_ = producer.Close()
}

func TestNewConsumer(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")
	if err := os.WriteFile(filename, []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	consumer, err := NewConsumer(filename)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	defer consumer.Close()

	if consumer.file == nil {
		t.Error("consumer.file is nil")
	}
	if consumer.decoder == nil {
		t.Error("consumer.decoder is nil")
	}
}

func TestNewConsumer_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "nonexistent.log")

	_, err := NewConsumer(filename)
	if err == nil {
		t.Error("NewConsumer() expected error for nonexistent file")
	}
}

func TestConsumer_ReadEvent(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")

	producer, err := NewProducer(filename)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}
	event := &Event{ID: 1, CarModel: "Lada", Price: 400000}
	if err := producer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}
	if err := producer.Close(); err != nil {
		t.Fatalf("Producer Close() error = %v", err)
	}

	consumer, err := NewConsumer(filename)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	defer consumer.Close()

	read, err := consumer.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}
	if read.ID != event.ID || read.CarModel != event.CarModel || read.Price != event.Price {
		t.Errorf("ReadEvent() = %+v, want %+v", read, event)
	}
}

func TestConsumer_Close(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")
	if err := os.WriteFile(filename, []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	consumer, err := NewConsumer(filename)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}

	if err := consumer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestProducerConsumer_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "events.log")

	producer, err := NewProducer(filename)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}

	for _, event := range events {
		if err := producer.WriteEvent(event); err != nil {
			t.Fatalf("WriteEvent() error = %v", err)
		}
	}
	if err := producer.Close(); err != nil {
		t.Fatalf("Producer Close() error = %v", err)
	}

	consumer, err := NewConsumer(filename)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	defer consumer.Close()

	for i, want := range events {
		read, err := consumer.ReadEvent()
		if err != nil {
			t.Fatalf("ReadEvent() #%d error = %v", i, err)
		}
		if read.ID != want.ID || read.CarModel != want.CarModel || read.Price != want.Price {
			t.Errorf("ReadEvent() #%d = %+v, want %+v", i, read, want)
		}
	}
}
