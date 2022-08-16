# Lab 3 Fault-tolerant Key/Value Service

## Part A: Key/value service without snapshots

### TASK

Your first task is to implement a solution that works when there are no dropped messages, and no failed servers.

You'll need to add RPC-sending code to the Clerk Put/Append/Get methods in `client.go`, and implement `PutAppend()` and `Get()` RPC handlers in `server.go`. These handlers should enter an `Op` in the Raft log using `Start()`; you should fill in the `Op` struct definition in `server.go` so that it describes a Put/Append/Get operation. Each server should execute `Op` commands as Raft commits them, i.e. as they appear on the `applyCh`. An RPC handler should notice when Raft commits its `Op`, and then reply to the RPC.

You have completed this task when you **reliably** pass the first test in the test suite: "One client".

由于使用的是 2022 版本的实验要求，相比于 2020 新增了 Lab 2D 也就是关于 snapshot 的部分，所以如果 Lab 2 的实现没有问题的话，在 Lab 3 中就不用对实现的 Raft 协议再做修改了。

在 Task 下面还有几条有用的 Hint，大致是：

- After calling `Start()`, your kvservers will need to wait for Raft to complete agreement. Commands that have been agreed upon arrive on the `applyCh`. Your code will need to keep reading `applyCh` while `PutAppend()` and `Get()` handlers submit commands to the Raft log using `Start()`. Beware of deadlock between the kvserver and its Raft library. 

  这条说明了 KVServer 的工作方式：通过调用 Start 先把指令写入 raft 日志达成共识，然后通过 Apply channel 接受消息运行指令。

- A kvserver should not complete a `Get()` RPC if it is not part of a majority (so that it does not serve stale data). A simple solution is to enter every `Get()` (as well as each `Put()` and `Append()`) in the Raft log. You don't have to implement the optimization for read-only operations that is described in Section 8.
  KVServer 是需要实现线性一致读的，一个简单的实现方式是读请求也写入到 Log 里面。论文的 Section 8 提供了两种实现的方法：ReadIndex Read 和 Lease Read。这里可以参见 [这篇博客](https://www.sofastack.tech/blog/sofa-jraft-linear-consistent-read-implementation/)。

### Implementation

有关这一部分的实现，在  Raft [作者博士论文](https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf) 的 6.3小节——实现线性化语义做了详细的介绍。简单记录一下实现中需要注意的一些问题，具体实现见代码。

#### Client

在 Client 部分我们需要解决的问题是：客户端发送请求，服务端commit成功并且apply，然后返回结果给client。但是返回结果的RPC包丢失了，client只能重新发送这个请求，知道收到一个返回结果为止。但是这个重发的请求在服务端已经应用过了，如果不加处理，就违背了一致性。

作者给出的解决方法是，用clientId和commandId来唯一标识一个客户端，从而保证线性一致性。日志可以被 commit 多次，但是只能 apply 一次。可以参考这个知乎回答：[raft在处理用户请求超时的时候，如何避免重试的请求被多次应用？](https://www.zhihu.com/question/278551592/answer/400962941)

此处clientId使用的是实验本身给出的nrand函数。在生产环境中一般使用uuid或者snowflake这种作为独特标识。在实验中只要不重复即可。

还有需要注意的是，当client请求的server不是leader的时候，需要改变请求节点。

#### Server

服务端的实现稍复杂一些。

首先是KV服务，在实现中把应用抽象成为KVStateMachine接口，然后实现了一个内存KV，可以满足实验要求。

然后是处理模型。对于server来说，它的任务是在接受来自client的请求之后，首先将请求commit到raft log中，等待 raft 把 ApplyMsg 推送到 ApplyCh之后再应用到状态机中，然后从状态机获得输出返回给客户端。

在实现中，server 协程将日志放入raft层同步后注册一个channel来阻塞等待，接着apply协程监控applyCh，在raft层已经commit日志之后，apply协程将首先将其应用到状态机中，然后根据index将结果推送到注册的channel中，这使得server可以解除阻塞并回复给客户端。

对于非读请求，需要记录 lastOperation，以clientId为索引，其中的信息包括MaxAppliedCommandId 以及 lastReply，以此来进行去重，并返回结果。



在实现中还有一些细节需要注意 / 优化：

- 提交日志时不持有kvserver的锁，raft本身可以保证并发的正确性；
- 在server阻塞等待apply的返回时，需要考虑超时重试
- apply日志需要防止状态机回退，在applyCh中获取消息之后需要检查index和lastApplied大小关系
- 推送到raft之前可以先尝试去重，对于非读请求检查其commandId是否小于lastOperation中记录的maxAppliedCommandId

## Part B: Key/value service with snapshots

### TASK

Modify your kvserver so that it detects when the persisted Raft state grows too large, and then hands a snapshot to Raft. When a kvserver server restarts, it should read the snapshot from `persister` and restore its state from the snapshot.

### Implementation

如果 Lab 2D 中的 snapshot 实现没有问题的话，这一部分的实现就比较简单了。只需要对kvserver中的以下两个部分做持久化保存即可：

- 状态机
- lastOperation

持久化又分为两种情况：

- 主动的持久化：在 raft 状态增加的时候，也就是接收到 commandValid 为 true 的ApplyMsg，主动检查 raftStateSIze 是否超过 maxraftstate
- 被动的持久化：在接收到 snapshotValid 为true 的 ApplyMsg 时，说明 leader 发来了安装快照的信息，调用 condInstallSnapshot，并根据其返回确认是否安装该快照