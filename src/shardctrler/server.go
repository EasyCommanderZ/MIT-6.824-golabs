package shardctrler

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
)

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32

	// Your data here.
	lastOperations map[int64]OperationInfo
	notifyChans    map[int]chan *CommandReply
	// configs []Config // indexed by config num
	stateMachine ConfigStateMachine
}

type Op struct {
	// Your data here.
}

func (sc *ShardCtrler) Join(args *JoinArgs, reply *JoinReply) {
	// Your code here.
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *LeaveReply) {
	// Your code here.
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *MoveReply) {
	// Your code here.
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *QueryReply) {
	// Your code here.
}

func (sc *ShardCtrler) Command(args *CommandArgs, reply *CommandReply) {
	sc.mu.Lock()
	if args.OP != OP_QUERY && sc.isDuplicatedRequest(args.ClientId, args.CommandId) {
		LastReply := sc.lastOperations[args.ClientId].LastReply
		reply.Config, reply.Err = LastReply.Config, LastReply.Err
		sc.mu.Unlock()
		return
	}
	sc.mu.Unlock()
	index, _, isLeader := sc.rf.Start(Command{args})
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	sc.mu.Lock()
	ch := sc.getNotifyChan(index)
	sc.mu.Unlock()
	select {
	case result := <-ch:
		reply.Config, reply.Err = result.Config, result.Err
	case <-time.After(ExecuteTimeout):
		reply.Err = ErrTimeout
	}
	go func() {
		sc.mu.Lock()
		sc.removeOutdataedNotifyChan(index)
		sc.mu.Unlock()
	}()
}

func (sc *ShardCtrler) applier() {
	for !sc.killed() {
		select {
		case msg := <-sc.applyCh:
			if msg.CommandValid {
				var reply *CommandReply
				command := msg.Command.(Command)
				sc.mu.Lock()
				if command.OP != OP_QUERY && sc.isDuplicatedRequest(command.ClientId, command.CommandId) {
					reply = sc.lastOperations[command.ClientId].LastReply
				} else {
					reply = sc.applyLogToStateMachine(command)
					if command.OP != OP_QUERY {
						sc.lastOperations[command.ClientId] = OperationInfo{
							MaxAppliedCommandId: command.CommandId,
							LastReply:           reply,
						}
					}
				}
				if currentTerm, isLeader := sc.rf.GetState(); isLeader && msg.CommandTerm == currentTerm {
					ch := sc.getNotifyChan(msg.CommandIndex)
					ch <- reply
				}
				sc.mu.Unlock()
			} else {
				panic(fmt.Sprintf("unexpected Message %v", msg))
			}
		}
	}
}

// the tester calls Kill() when a ShardCtrler instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (sc *ShardCtrler) Kill() {
	atomic.StoreInt32(&sc.dead, 1)
	sc.rf.Kill()
	// Your code here, if desired.
}

func (sc *ShardCtrler) killed() bool {
	return atomic.LoadInt32(&sc.dead) == 1
}

// needed by shardkv tester
func (sc *ShardCtrler) Raft() *raft.Raft {
	return sc.rf
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant shardctrler service.
// me is the index of the current server in servers[].
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardCtrler {
	sc := new(ShardCtrler)
	sc.me = me
	sc.dead = 0

	labgob.Register(Command{})
	sc.applyCh = make(chan raft.ApplyMsg)
	sc.rf = raft.Make(servers, me, persister, sc.applyCh)

	// Your code here.
	sc.stateMachine = NewMemoryConfigStateMachine()
	sc.lastOperations = make(map[int64]OperationInfo)
	sc.notifyChans = make(map[int]chan *CommandReply)
	go sc.applier()
	return sc
}
