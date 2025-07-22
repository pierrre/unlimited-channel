package unlimitedchannel

const queuePoolMaxSize = 1000

type queue[T any] struct {
	head *queueElement[T]
	tail *queueElement[T]

	elemPool []*queueElement[T]
}

func (q *queue[T]) enqueue(value T) {
	var newElem *queueElement[T]
	lp := len(q.elemPool)
	if lp > 0 {
		newElem = q.elemPool[lp-1]
		q.elemPool[lp-1] = nil
		q.elemPool = q.elemPool[:lp-1]
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
	if len(q.elemPool) < queuePoolMaxSize {
		q.elemPool = append(q.elemPool, oldElem)
	}
	return value, true
}

type queueElement[T any] struct {
	value T
	next  *queueElement[T]
}
