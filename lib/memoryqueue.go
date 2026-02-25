package lib

import (
	"errors"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryQueue 是一个支持动态扩容的循环队列（线程安全）
type MemoryQueue struct {
	data        []string
	mu          *sync.RWMutex
	defaultSize int
	maxCapacity int
	head        int
	tail        int
	size        int
	count       int
}

// NewMemoryQueue 创建一个指定初始大小的内存队列
func NewMemoryQueue() *MemoryQueue {
	que := &MemoryQueue{
		head:  0,
		tail:  0,
		count: 0,
		mu:    &sync.RWMutex{},
	}

	var defaultSize int = 1024 * 1024
	que.defaultSize = defaultSize
	que.size = defaultSize
	que.data = make([]string, defaultSize)

	var maxCapacity int = int(que.memAvailable() / 3 / 200) // 可用内存的1/3, 队列元素长度:平均200个字节
	que.maxCapacity = maxCapacity

	return que
}

// 只有在队列为空时, 方可调用 SetSize, 当队列有数据时,不可重置size
func (q *MemoryQueue) SetSize(newSize int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if newSize < 1 {
		return errors.New("size must great than 1")
	}
	if q.head > 0 || q.tail > 0 || q.count > 0 {
		return errors.New("set size must before queue empty")
	}
	q.size = newSize
	q.data = make([]string, newSize)

	return nil
}

func (q *MemoryQueue) SetMaxCapacity(newMaxCapacity int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if newMaxCapacity < q.size {
		return errors.New("maxCapacity must  great than size")
	}
	if q.head > 0 || q.tail > 0 || q.count > 0 {
		return errors.New("set maxCapacity must before queue empty")
	}

	q.maxCapacity = newMaxCapacity
	return nil
}

// Add 向队列添加元素。如果队列已满，则自动扩容。
func (q *MemoryQueue) Add(item string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size < 1 {
		return errors.New("size must great than 1")
	}

	// 队列已满，进行扩容
	if q.count == q.size {
		err := q.resize()
		if err != nil {
			return err
		}
	}

	q.data[q.tail] = item
	q.tail = (q.tail + 1) % q.size
	q.count++
	return nil
}

// resize 扩容队列。此方法只在持锁的情况下调用。
func (q *MemoryQueue) resize() error {
	newSize := q.size * 2
	if newSize < q.size { // int 类型溢出
		return errors.New("queue size reached limit of type int")
	} else if newSize >= q.maxCapacity {
		return errors.New("queue size reached max capacity")
	}

	newData := make([]string, newSize)

	// 使用循环复制以确保正确性，避免因 head/tail 关系复杂而导致的 bug
	for i := 0; i < q.count; i++ {
		newData[i] = q.data[(q.head+i)%q.size]
	}

	// 更新队列的元数据
	q.data = newData
	q.head = 0
	q.tail = q.count
	q.size = newSize

	runtime.GC() // 强制垃圾回收

	return nil
}

// Get 从队列中获取元素。如果队列为空，则返回 false。
func (q *MemoryQueue) Get() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		return "", false // 队列为空
	}

	item := q.data[q.head]
	// 清除旧元素引用，帮助垃圾回收
	q.data[q.head] = ""
	q.head = (q.head + 1) % q.size
	q.count--

	return item, true
}

// HasMore 判断队列是否还有元素
func (q *MemoryQueue) HasMore() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.count > 0
}

// Len 返回当前队列中有效元素个数
func (q *MemoryQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.count
}

// GetQueueSize 返回队列的当前大小
func (q *MemoryQueue) GetQueueSize() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size
}

// Reset 清空队列内容，但保留底层切片，以供复用。
func (q *MemoryQueue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 清空切片中的元素，避免内存泄漏
	for i := 0; i < q.count; i++ {
		index := (q.head + i) % q.size
		q.data[index] = ""
	}

	q.head = 0
	q.tail = 0
	q.count = 0
}

// Close 可以配合 future 扩展使用
func (q *MemoryQueue) Close() error {
	return nil // placeholder, 无资源需要释放
}

func (q *MemoryQueue) memAvailable() uint64 {
	var defaultSize uint64 = 102400
	// 内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return defaultSize
	}

	return memInfo.Available
}
