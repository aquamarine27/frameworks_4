package state

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Состояния машины состояний
type State string

const (
	StateNew                 State = "Новый"
	StateApplicationAccepted State = "ЗаявкаПринята"
	StateResourceBooked      State = "РесурсЗабронирован"
	StateAccessGranted       State = "ДоступВыдан"
	StateCompleted           State = "Завершён"
	StateCompensationDone    State = "КомпенсацияВыполнена"
	StateError               State = "Ошибка"
)

// События (переходы)
type Event string

const (
	EventAcceptApplication Event = "ПринятьЗаявку"
	EventBook              Event = "Забронировать"
	EventGrantAccess       Event = "ВыдатьДоступ"
	EventComplete          Event = "Завершить"
)

// конкретный процесс
type Process struct {
	Key             string
	State           State
	CreatedAt       time.Time
	UpdatedAt       time.Time
	IdempotencyKeys map[string]bool
}

// все процессы в памяти
type Store struct {
	mu                sync.RWMutex
	processes         map[string]*Process
	FailOnGrantAccess bool
}

func NewStore() *Store {
	return &Store{
		processes: make(map[string]*Process),
	}
}

// создание нового или возвращение текущего
func (s *Store) GetOrCreate(processKey string) *Process {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.processes[processKey]; ok {
		return p
	}
	p := &Process{
		Key:             processKey,
		State:           StateNew,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		IdempotencyKeys: make(map[string]bool),
	}
	s.processes[processKey] = p
	return p
}
func (s *Store) Get(processKey string) *Process {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processes[processKey]
}

// возвращение ошибки
func (s *Store) CheckAndRegisterIdempotency(processKey, idempotencyKey string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.processes[processKey]
	if !ok {
		return false, fmt.Errorf("процесс %q не найден", processKey)
	}
	if p.IdempotencyKeys[idempotencyKey] {
		return true, nil // повтор
	}
	p.IdempotencyKeys[idempotencyKey] = true
	return false, nil // первый раз
}

type TransitionResult struct {
	PrevState    State
	NextState    State
	Compensated  bool
	ErrorMessage string
}

func (s *Store) ApplyEvent(processKey string, event Event, failOnGrantAccess bool) (*TransitionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.processes[processKey]
	if !ok {
		return nil, fmt.Errorf("процесс %q не найден", processKey)
	}

	prev := p.State
	res := &TransitionResult{PrevState: prev}

	switch event {
	case EventAcceptApplication:
		if prev != StateNew {
			return nil, fmt.Errorf("недопустимый переход: %s -[%s]-> ?", prev, event)
		}
		p.State = StateApplicationAccepted

	case EventBook:
		if prev != StateApplicationAccepted {
			return nil, fmt.Errorf("недопустимый переход: %s -[%s]-> ?", prev, event)
		}
		p.State = StateResourceBooked

	case EventGrantAccess:
		if prev != StateResourceBooked {
			return nil, fmt.Errorf("недопустимый переход: %s -[%s]-> ?", prev, event)
		}
		if failOnGrantAccess {
			p.State = StateCompensationDone
			res.Compensated = true
			res.ErrorMessage = "имитация сбоя на шаге ВыдатьДоступ — компенсация выполнена"
			p.UpdatedAt = time.Now()
			res.NextState = p.State
			return res, errors.New(res.ErrorMessage)
		}
		p.State = StateAccessGranted

	case EventComplete:
		if prev != StateAccessGranted {
			return nil, fmt.Errorf("недопустимый переход: %s -[%s]-> ?", prev, event)
		}
		p.State = StateCompleted

	default:
		return nil, fmt.Errorf("неизвестное событие: %s", event)
	}

	p.UpdatedAt = time.Now()
	res.NextState = p.State
	return res, nil
}

// переводит процесс в состояние Ошибка
func (s *Store) SetError(processKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.processes[processKey]; ok {
		p.State = StateError
		p.UpdatedAt = time.Now()
	}
}

// возвращает все процессы
func (s *Store) List() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		result = append(result, p)
	}
	return result
}
