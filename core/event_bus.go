package core

import (
	"log/slog"
	"sync"
)

type Event interface {
	Name() string
}

type EventHandler interface {
	Handle(event Event)
}

type EventBus interface {
	Logger() *slog.Logger
	Subscribe(eventName string, handler EventHandler)
	Dispatch(event Event)
}

type EventBusImpl struct {
	handlers map[string][]EventHandler
	rwMutex  sync.RWMutex
	Log      *slog.Logger
}

func (bus *EventBusImpl) Logger() *slog.Logger {
	return bus.Log
}
func (bus *EventBusImpl) Subscribe(eventName string, handler EventHandler) {
	bus.rwMutex.Lock()
	defer bus.rwMutex.Unlock()
	bus.handlers[eventName] = append(bus.handlers[eventName], handler)
	bus.Logger().Info("subscribed handler to event", "event", eventName)
}

func (bus *EventBusImpl) Dispatch(event Event) {
	bus.rwMutex.RLock()
	defer bus.rwMutex.RUnlock()
	for _, handler := range bus.handlers[event.Name()] {
		bus.Logger().Debug("dispatching event to handler", "event", event.Name())
		go handler.Handle(event)
	}
}

func NewEventBus(log *slog.Logger) EventBus {
	return &EventBusImpl{
		handlers: make(map[string][]EventHandler),
		Log:      log,
	}
}
