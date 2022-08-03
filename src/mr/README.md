# Lab1 MapReduce

### Keypoint

- 系统中的 worker 设计为无状态的，coordinator 只关心任务的状态，并且当前系统的任务状态由 coordinator来维护。这也意味着 worker 的地位是对等的，它只负责向 coordinator 申请任务并且执行，根据任务的 taskType 进行判断自己应该做什么

- 根据这种设计，map 和 reduce 的结果文件名需要设计才能保证作业完成。中间结果的合并由 coordinator 来完成，关键字涉及到 mapId 、 reduceId，再加上他们的 taskType 来表示一个对应的结果。

- worker 在完成任务后，需要向 coordinator report 以更新任务的状态

- coordinator 要定期检查超时的任务，这个列表由它自己维护。这个已分发的任务列表会在收到 report 的时候更新

## Detail Implementation

### RPC Struct

RPC通信部分的主要有如下几个结构的设计：

- TaskInfo

  ```go
  type TaskInfo struct {
  	Id       int // 任务的ID
  	TaskType string // 任务的种类，有 MAP REDUCE QUIT 三种
  	Filename string // 使用的文件名
  	Deadline time.Time // 任务过期时间
  }
  ```

  需要说明的是，在 Coordinator 中，我使用 **taskType_Id** 这样的命名形式来标识一个特定的任务。并且在不同种类的任务中，需要的参数也不完全相同。

- AssignTaskArgs && AssignTaskReply

  用于 Worker 申请任务时的通信结构。

  ```go
  type AssignTaskArgs struct {
  }
  
  type AssignTaskReply struct {
  	Info    TaskInfo
  	NMap    int
  	NReduce int
  }
  ```

  由于 Worker 被设计成为无状态的，所以各个 Worker 的地位等价，并不需要特别的申请参数提供给 Coordinator 进行类似身份证明的操作。

  Coordinator 的回复中包含了整个 MapReduce 工作中 Map 和 Reduce 分别的工作数量，以及当前 Task 的信息。

- ReportTaskArgs && ReportTaskReply

  用户 Worker 向 Coordinator 通报任务状态的通信结构。

  ```go
  type ReportTaskArgs struct {
  	TaskID   int
  	TaskType string
  }
  
  type ReportTaskReply struct {
  	Accept bool
  }
  ```

  前面说到，在 Coordinator 中，使用 **taskType_Id** 这样的命名形式来标识一个特定的任务，因此只需要任务的 Id 以及任务的种类即可通报任务的完成情况，Coordinator 也只需要回复是否接受当前的任务结果。

### Worker

Worker 的运行函数中，首先它循环地向 Coordinator 申请任务，在收到分配的任务后，根据任务的种类运行要求的任务内容。在任务完成之后，向 Coordinator 通报任务的结果。这一部分的设计比较简单，具体的实现可以看代码。其中有几点需要说明：

- 首先是文件的命名。在运行 MAP 或者 REDUCE 任务时，Worker 生成的文件其文件名是一个临时的文件名。只有当 Coordinator 接受当前任务运行的结果之后，才会由 Coordinator 对其进行重命名的操作，以作为正式投喂给REDUCE任务的文件或者输出文件。
- 对于文件的创建，使用的是 `os.CreateTemp`创建一个临时文件，等当前结果写完之后，再对临时文件进行重命名作为结果。这一实现是为了应对测试脚本中的 crash test

### Coordinator

这一部分的设计和实现相对复杂一些。首先需要明确 Coordinator 的功能：

1. 响应 Worker 申请任务的请求，也就是向 Worker 分发任务
2. 接收来自 Worker 对于任务执行的回报，也就是判断分发出去的任务的运行状态，确定任务是已经完成还是需要回收之后重新分配执行
3. 在完成某一阶段的所有任务后，能够切换当前的工作阶段，如从MAP切换到REDUCE
4. 能够定时的回收超时的已分配任务

除此之外，还需要意识到，虽然 Worker 只关心任务的状态，但是它需要面对的是未知数量的 Worker，也就意味着 Coordinator 在同一时间又可能需要处理多个请求，这就涉及到了数据竞争的问题。

根据这些需求，按照如下的方式设计 Coordinator ：

```go
type Coordinator struct {
	nMap    int
	nReduce int

	taskQueue    chan TaskInfo // 任务列表，使用 channel 管理
	taskTraceMap map[string]TaskInfo // 已分发的任务，使用 taskType + Id 进行标识
  
	Phase string // Coordinator 的运行阶段
	lck   sync.Mutex // 互斥量，用于进行数据同步
}
```

具体的实现见代码。有以下几点需要说明：

- 在 Coordinator 的初始化方法中，设定好了 nMap 和 nReduce，也就是两个阶段的任务数量。并且在开始响应请求之前，就向 taskQueue 中预先传入了 MAP 任务。需要注意的是，这些预设任务的 Deadline 只有在分配出去的时候才会更新
- 在 Coordinator 开始响应之后，也启动了一个 goroutine 用于定时的回收过期任务
- 分配任务时，Coordinator 根据当前自己的运行阶段来确定任务在 taskTraceMap 中的 key。在接受到 Worker 申请任务的请求之后， Coordinator 把任务从 taskQueue 中取出，更新任务的 Deadline，并发送给Worker，并且在 taskTraceMap 中使用 key 来记录这个任务是已经分配的
- 接受任务通报时，Coordinator 首先检查任务是否已经超时。如果任务超时，交给超时的协程进行处理即可，如果没有超时，则进行接受响应。根据任务的类型，处理他们的临时文件作为正式结果，并且从 taskTraceMap 中移除对这个任务的跟踪。
- 由于 Coordinator 是通过自身的运行阶段来记录任务的，需要设计一个运行时切换的方法。其判断依据是 taskQueue 和 taskTraceMap 都为空，这样才能表示当前的工作任务已经结束，可以切换到下一阶段
- 特别需要注意的是，以上提到的所有关于 taskQueue 和 taskTraceMap 的访问，都需要注意数据的同步问题，也就是需要适当的进行加锁解锁。并且需要注意，channel 的访问是会被阻塞的，会影响锁的申请和释放。
