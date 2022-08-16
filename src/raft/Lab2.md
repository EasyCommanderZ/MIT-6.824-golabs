# Lab 2

## Lab 2A

### Task

Implement Raft leader election and heartbeats (`AppendEntries` RPCs with no log entries). The goal for Part 2A is for a single leader to be elected, for the leader to remain the leader if there are no failures, and for a new leader to take over if the old leader fails or if packets to/from the old leader are lost. Run `go test -run 2A` to test your 2A code.

---

这一部分是实现 Leader Election 以及发送 heartbeats。在实现之前，始终记住一个重要的规则：

**STICK TO FIGURE 2 IN RAFT PAPER**

根据前人的经验，论文在 Figure 2 中给的定义是非常严格的。如果能够完整复现，就能满足实验中的要求。

> 论文见 ： http://nil.csail.mit.edu/6.824/2020/papers/raft-extended.pdf

---

