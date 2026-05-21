package audit

import (
	"context"
	"log"
	"time"
)

const defaultAuditTimeout = 10 * time.Second

// Observer — приёмник событий аудита (паттерн «Наблюдатель»).
type Observer interface {
	// OnAudit обрабатывает одно событие; ошибка логируется издателем, но не прерывает других наблюдателей.
	OnAudit(ctx context.Context, ev Event) error
}

// Publisher — субъект: рассылает событие всем зарегистрированным наблюдателям.
type Publisher struct {
	observers []Observer
}

// NewPublisher собирает изображение субъекта из наблюдателей. Без наблюдателей возвращает nil.
func NewPublisher(observers ...Observer) *Publisher {
	if len(observers) == 0 {
		return nil
	}
	return &Publisher{observers: append([]Observer(nil), observers...)}
}

// Publish уведомляет всех наблюдателей асинхронно (не блокирует HTTP-ответ).
func (p *Publisher) Publish(ev Event) {
	if p == nil {
		return
	}
	for _, o := range p.observers {
		obs := o
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), defaultAuditTimeout)
			defer cancel()
			if err := obs.OnAudit(ctx, ev); err != nil {
				log.Printf("audit: observer failed: %v", err)
			}
		}()
	}
}
