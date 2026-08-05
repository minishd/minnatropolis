package emitter

import (
	"slices"
	"sync"
)

const (
	kTopics = "em-topics"
)

type (
	Metadata interface {
		Store(key string, value any)
		Load(key string) (value any, ok bool)
		Delete(key string)
	}

	Subscriber[I comparable] interface {
		GetMetadata() Metadata
		GetSubscriberID() I
	}
)

type subscription[I comparable, S Subscriber[I]] struct {
	sub S
	cb  func(any)
}

type topic[I comparable, S Subscriber[I]] struct {
	mu     sync.RWMutex
	subscs map[I]subscription[I, S]
}

type Emitter[I comparable, S Subscriber[I]] struct {
	mu     sync.Mutex
	topics map[string]*topic[I, S]
}

func New[I comparable, S Subscriber[I]]() *Emitter[I, S] {
	return &Emitter[I, S]{
		topics: make(map[string]*topic[I, S]),
	}
}

func (e *Emitter[I, S]) getTopic(ktopic string) *topic[I, S] {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.topics[ktopic]
	if ok {
		return t
	}

	newT := &topic[I, S]{
		subscs: make(map[I]subscription[I, S]),
	}
	e.topics[ktopic] = newT
	return newT
}

type topicsStore struct {
	mu    sync.Mutex
	store []string
}

func getTopics[I comparable, S Subscriber[I]](sub S) *topicsStore {
	ts, ok := sub.GetMetadata().Load(kTopics)
	if ok {
		return ts.(*topicsStore)
	} else {
		ts := &topicsStore{}
		sub.GetMetadata().Store(kTopics, ts)
		return ts
	}
}

func (e *Emitter[I, S]) MakeSub(sub S, ktopic string, cb func(any)) {
	t := e.getTopic(ktopic)
	id := sub.GetSubscriberID()

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.subscs[id]; ok {
		return
	}

	ts := getTopics(sub)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t.subscs[id] = subscription[I, S]{
		sub: sub,
		cb:  cb,
	}
	ts.store = append(ts.store, ktopic)
}

func (e *Emitter[I, S]) RemoveSub(sub S, ktopic string) {
	t := e.getTopic(ktopic)
	id := sub.GetSubscriberID()

	ts := getTopics(sub)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	delete(t.subscs, id)
	ts.store = slices.DeleteFunc(ts.store, func(k string) bool { return k == ktopic })
}

func (e *Emitter[I, S]) DestroySub(sub S) {
	ts := getTopics(sub)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, ktopic := range ts.store {
		t := e.getTopic(ktopic)
		t.mu.Lock()
		delete(t.subscs, sub.GetSubscriberID())
		t.mu.Unlock()
	}
}

func (e *Emitter[I, S]) GetSubs(ktopic string) (subs []S) {
	t := e.getTopic(ktopic)
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, subsc := range t.subscs {
		subs = append(subs, subsc.sub)
	}

	return
}

func (e *Emitter[I, S]) Publish(ktopic string, msg any) {
	t := e.getTopic(ktopic)
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, subsc := range t.subscs {
		subsc.cb(msg)
	}
}
