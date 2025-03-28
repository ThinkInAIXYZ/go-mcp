package pkg

import (
	"container/list"
	"sync"
	"time"
)

// TimeWheel 时间轮结构，用于高效管理基于时间的任务调度
type TimeWheel struct {
	// 时间轮的基本参数
	interval time.Duration // 时间轮的最小时间单位（一个槽位的时间跨度）
	ticker   *time.Ticker  // 定时器，用于触发时间轮的转动
	slots    []*taskBucket // 时间轮的槽位，每个槽位是一个任务桶

	// 当前时间轮的状态
	currentPos  int           // 当前指针位置
	slotNum     int           // 槽位数量
	addTaskC    chan *Task    // 添加任务的通道
	removeTaskC chan string   // 移除任务的通道
	stopC       chan struct{} // 停止时间轮的通道

	// 时间轮的层级关系
	parent     *TimeWheel    // 父级时间轮，用于处理超出当前时间轮范围的任务
	maxTimeout time.Duration // 当前时间轮能处理的最大超时时间

	// 任务执行器
	executor func(*Task) // 任务执行函数

	// 并发控制
	sync.RWMutex
}

// 任务桶，存储在同一个槽位的所有任务
type taskBucket struct {
	tasks map[string]*list.Element // 任务ID到列表元素的映射
	list  *list.List               // 双向链表，存储实际的任务
	sync.RWMutex
}

// 创建新的任务桶
func newTaskBucket() *taskBucket {
	return &taskBucket{
		tasks: make(map[string]*list.Element),
		list:  list.New(),
	}
}

// Task 表示一个定时任务
type Task struct {
	ID         string        // 任务唯一标识
	Delay      time.Duration // 延迟执行时间
	Data       interface{}   // 任务数据
	round      int           // 任务需要经过的轮数
	expiration time.Time     // 任务的过期时间点
}

// NewTimeWheel 创建一个新的时间轮
func NewTimeWheel(interval time.Duration, slotNum int, executor func(*Task)) *TimeWheel {
	if interval <= 0 || slotNum <= 0 || executor == nil {
		return nil
	}

	tw := &TimeWheel{
		interval:    interval,
		slots:       make([]*taskBucket, slotNum),
		currentPos:  0,
		slotNum:     slotNum,
		addTaskC:    make(chan *Task, 100),
		removeTaskC: make(chan string, 100),
		stopC:       make(chan struct{}),
		executor:    executor,
		maxTimeout:  interval * time.Duration(slotNum),
	}

	// 初始化每个槽位的任务桶
	for i := 0; i < slotNum; i++ {
		tw.slots[i] = newTaskBucket()
	}

	return tw
}

// Start 启动时间轮
func (tw *TimeWheel) Start() {
	tw.ticker = time.NewTicker(tw.interval)
	go tw.run()
}

// Stop 停止时间轮
func (tw *TimeWheel) Stop() {
	tw.stopC <- struct{}{}
}

// run 时间轮的主循环
func (tw *TimeWheel) run() {
	defer func() {
		if tw.ticker != nil {
			tw.ticker.Stop()
			tw.ticker = nil
		}
	}()

	for {
		select {
		case <-tw.ticker.C: // 时间轮转动
			tw.tickHandler()
		case task := <-tw.addTaskC: // 添加任务
			tw.addTask(task)
		case taskID := <-tw.removeTaskC: // 移除任务
			tw.removeTask(taskID)
		case <-tw.stopC: // 停止时间轮
			return
		}
	}
}

// tickHandler 处理时间轮的每一次转动
func (tw *TimeWheel) tickHandler() {
	tw.Lock()
	currentBucket := tw.slots[tw.currentPos]
	tw.Unlock()

	tw.scanAndRunTasks(currentBucket)

	tw.Lock()
	// 移动时间轮指针
	if tw.currentPos == tw.slotNum-1 {
		tw.currentPos = 0
	} else {
		tw.currentPos++
	}
	tw.Unlock()
}

// scanAndRunTasks 扫描并执行当前槽位中到期的任务
func (tw *TimeWheel) scanAndRunTasks(bucket *taskBucket) {
	bucket.Lock()
	defer bucket.Unlock()

	for e := bucket.list.Front(); e != nil; {
		task := e.Value.(*Task)

		if task.round > 0 { // 任务还需要经过多轮
			task.round--
			e = e.Next()
			continue
		}

		// 任务可以执行了
		next := e.Next()
		bucket.list.Remove(e)
		delete(bucket.tasks, task.ID)
		e = next

		// 异步执行任务，避免阻塞时间轮
		go tw.executor(task)
	}
}

// addTask 添加任务到时间轮
func (tw *TimeWheel) addTask(task *Task) {
	if task.Delay < 0 {
		task.Delay = 0
	}

	// 设置任务的过期时间
	task.expiration = time.Now().Add(task.Delay)

	// 如果任务的延迟超过了当前时间轮的最大超时时间，且有父时间轮
	if task.Delay > tw.maxTimeout && tw.parent != nil {
		tw.parent.addTaskC <- task
		return
	}

	// 计算任务应该放在哪个槽位
	pos, round := tw.getPositionAndRound(task.Delay)
	task.round = round

	// 获取槽位的任务桶
	bucket := tw.slots[pos]

	bucket.Lock()
	// 如果任务已存在，先移除旧任务
	if elem, ok := bucket.tasks[task.ID]; ok {
		bucket.list.Remove(elem)
		delete(bucket.tasks, task.ID)
	}

	// 添加新任务
	elem := bucket.list.PushBack(task)
	bucket.tasks[task.ID] = elem
	bucket.Unlock()
}

// getPositionAndRound 计算任务应该放在哪个槽位以及需要经过多少轮
func (tw *TimeWheel) getPositionAndRound(delay time.Duration) (int, int) {
	// 计算需要多少个时间单位
	ticks := int(delay / tw.interval)

	// 计算轮数和位置
	round := ticks / tw.slotNum
	pos := (tw.currentPos + ticks) % tw.slotNum

	return pos, round
}

// removeTask 从时间轮中移除任务
func (tw *TimeWheel) removeTask(taskID string) {
	if taskID == "" {
		return
	}

	// 遍历所有槽位，查找并移除任务
	for _, bucket := range tw.slots {
		bucket.Lock()
		if elem, ok := bucket.tasks[taskID]; ok {
			bucket.list.Remove(elem)
			delete(bucket.tasks, taskID)
			bucket.Unlock()
			return
		}
		bucket.Unlock()
	}

	// 如果当前时间轮没有找到，尝试在父时间轮中查找
	if tw.parent != nil {
		tw.parent.removeTaskC <- taskID
	}
}

// AddTask 添加任务（公开方法）
func (tw *TimeWheel) AddTask(id string, delay time.Duration, data interface{}) {
	task := &Task{
		ID:    id,
		Delay: delay,
		Data:  data,
	}
	tw.addTaskC <- task
}

// RemoveTask 移除任务（公开方法）
func (tw *TimeWheel) RemoveTask(id string) {
	tw.removeTaskC <- id
}

// SetParent 设置父级时间轮
func (tw *TimeWheel) SetParent(parent *TimeWheel) {
	tw.parent = parent
	tw.maxTimeout = tw.interval * time.Duration(tw.slotNum)
}

// NewHierarchicalTimeWheel 创建一个分层时间轮
// 参数:
// - baseInterval: 基础时间轮的时间间隔
// - baseSlotNum: 基础时间轮的槽位数量
// - levels: 时间轮的层级数量
// - executor: 任务执行函数
func NewHierarchicalTimeWheel(baseInterval time.Duration, baseSlotNum, levels int, executor func(*Task)) *TimeWheel {
	if levels <= 0 {
		levels = 1
	}

	// 创建基础时间轮
	var wheels []*TimeWheel
	for i := 0; i < levels; i++ {
		// 每一层的时间间隔是前一层的时间间隔乘以槽位数量
		interval := baseInterval
		if i > 0 {
			interval = wheels[i-1].interval * time.Duration(baseSlotNum)
		}

		wheel := NewTimeWheel(interval, baseSlotNum, executor)
		wheels = append(wheels, wheel)

		// 设置父级时间轮
		if i > 0 {
			wheels[i-1].SetParent(wheel)
		}
	}

	return wheels[0] // 返回最底层的时间轮
}
