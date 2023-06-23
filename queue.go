package unlimitedchannel

import (
	"sync"
)

type queue[T any] struct {
	head *queueElement[T]
	tail *queueElement[T]

	elemPool sync.Pool
}

func (q *queue[T]) enqueue(value T) {
	newElemItf := q.elemPool.Get()
	var newElem *queueElement[T]
	if newElemItf != nil {
		newElem = newElemItf.(*queueElement[T]) //nolint:forcetypeassert // The pool only contains *queueElement[T].
	} else {
		newElem = &queueElement[T]{}
	}
	newElem.value = value
	if q.head == nil {
		q.head = newElem
	}
	if q.tail != nil {
		q.tail.next = newElem
	}
	q.tail = newElem
}

func (q *queue[T]) dequeue() (T, bool) {
	if q.head == nil {
		var value T
		return value, false
	}
	oldElem := q.head
	value := oldElem.value
	q.head = oldElem.next
	if q.head == nil {
		q.tail = nil
	}
	var zero T
	oldElem.value = zero
	oldElem.next = nil
	q.elemPool.Put(oldElem)
	return value, true
}

func (q *queue[T]) pick() (T, bool) {
	if q.head == nil {
		var value T
		return value, false
	}
	return q.head.value, true
}

func (q *queue[T]) reset() {
	q.head = nil
	q.tail = nil
}

type queueElement[T any] struct {
	value T
	next  *queueElement[T]
}
