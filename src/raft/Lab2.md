# Lab 2

Lab2 是实现 一个除节点变更之外的 RAFT 协议。根据前人的经验，论文在 Figure 2 中给的定义是非常严格的。如果能够完整复现，就能满足实验中的要求。

**STICK TO FIGURE 2 IN RAFT PAPER**

> 论文见 ： http://nil.csail.mit.edu/6.824/2020/papers/raft-extended.pdf

---

Raft 协议主要通过两方面来保证集群节点间的线性一致性：

- 主节点选举
- 日志复制

根据论文中 raft 的设计，raft 需要维护一些变量。raft 结构体设计如下：

```go
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Lab 2A
	// State Info
	role           string
	currentTerm    int // 当前任期
	votedFor       int
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	// Lab 2B
	logs        []Entry
	commitIndex int
	LastApplied int

	// leader node
	nextIndex  []int
	matchIndex []int

	applyCh   chan ApplyMsg
	applyCond *sync.Cond
}
```

此外，在运行主进程的同时，还运行了两个协程：

- applier：用于向 applyCh 中推送提交的日志，并且保证同一日志 exactly once
- tiker：用于触发 heartbeat 和 election 的超时自动执行

## Leader Election

首先实现的是 Leader Election，主节点选举。基本参照 figure 2 的描述实现即可。需要注意的是，只有在 vote 被 grant 的时候才会重置 election timer。

在选举主节点期间主要有以下几个操作需要进行（都是在 figure 2 中要求的）：

- 检查是否已经投票
- 检查双方任期
- 检查双方 Log 长度
- 投票

选举的触发来自于 election timer 的超时。由于 timer 的超时时间各不相同，由最先超时的节点来发起选举流程。具体实现见代码。需要注意的细节比较多，但是都是 figure 2 中要求的。此外需要注意的有：

- 并行异步投票，不阻塞 ticker，这样 election timer 再次过期的时候才能顺利的发起新一轮选举
- 过期的请求可以直接丢弃

## Log Replication

日志复制是 raft 算法的核心，基于日志复制又有日志压缩和快照的功能实现。为了应对分布式系统中复杂的情况，细节会很多。

在实现了日志压缩和快照的功能之后，需要注意的是日志的存储结构已经发生了变化，会有日志编号不在本地存储的情况发生。这个时候就需要根据需求检查是否需要快照来让 raft peer 来跟上 leader 的日志进度。

在代码的实现中，日志的复制是在心跳包和选举成功的时候触发的，换言之，只有 leader 会触发日志复制的请求。

raft 日志随着运行时间的增加会面临节点日志过长的问题，这对于掉线节点、重启节点和新节点的日志重放并不友好。所以，对于日志复制，还需要实现日志压缩和快照两个功能。然而在实现这两个功能的过程中，也会出现一些复杂的情况。比如出现follower更新所需的日志在主节点上已经被压缩，这个时候只能通过快照的形式来更新follower，等等。

快照的操作的时间点是上层应用决定的，通过 condInstallSanpshot 来调用。

日志的 apply 操作采用的是异步 apply，有一个单独的 apply 协程去处理请求。其实比较直观的解决思路是在commitIndex增长的时候就把已commit但未apply的日志推送到 apply channel。但是这里会出现的问题是，由于push到apply channel 的时候是不能持锁的，那么lastApplied在没有push完之前就无法得到更新，所以发送到channel的日志可能会重复。虽然这一点可以通过上层应用的检查来解决，但是不够简洁，每一个不同的应用都需要重复的进行检查函数的设计。所以可以只用一个 applier 协程去处理这些请求，让他不断的把 已commit但未apply 的日志提交到 apply channel中，这样就可以保证每条日志只被推送一次，也可以保证raft的新日志提交和上层应用的状态机更新能够并行。在这个过程中需要注意两点：

- 由于推送日志的过程不持锁，所以需要有变量记录之前的commitIndex等数据，防止在推送过程中raft节点的对应状态发生了变化
- 防止与installSnapshot并发导致lastApplied回退：需要注意的是，在applier工作的过程中，可能会有快照的应用，那么上层状态机和raft模块就会发生替换，即发生状态的变更。如果此时这个snapshot之后还有一批旧的entry被提交到channel中，那么上层应用需要能够知道这些entry已经过时，不能apply，同时applier也应该取 lastApplied 和 commitIndex 中的较大值来防止回退。

---

### 回忆实现过程中的一些问题

- Leader Election 的实现时，上任之后应该马上广播心跳以确认领导力
- 在实现实验的前半部分的时候，没有考虑日志压缩的需求，所以没有对entries的结构做特殊的封装处理。现在想想，应该写成一个结构，里面储存当前内存中的日志，以及首部的日志号已经长度等等信息，这样在实现日志压缩的时候能够更方便一些
- 在 leader 调用 append entry的时候，需要判断记录中的目标follower的日志状况，因为可能出现follower需要的日志已经被压缩形成快照，不在内存了。这个时候就需要要求follower进行快照安装来赶上日志进度，由 follower 来判断快照的有效性并且回复。日志复制之后是更新 nextIndex 和 matchIndex 两个记录组，然后是 commit log以及 apply log
