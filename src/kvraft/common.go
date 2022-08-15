package kvraft

import (
	"bytes"
	"time"

	"6.824/labgob"
)

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeOut     = "ErrTimeOut"
)

const (
	OP_PUT    = "Put"
	OP_APPEND = "Append"
	OP_GET    = "Get"
)

type CommandArgs struct {
	Key       string
	Value     string
	Op        string
	ClientId  int64
	CommandId int64
}

type CommandReply struct {
	Err   Err
	Value string
}

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
}

type GetReply struct {
	Err   Err
	Value string
}

type OperationInfo struct {
	MaxAppliedCommandId int64
	LastReply           *CommandReply
}

type Command struct {
	*CommandArgs
}

const ExecuteTimeout = 500 * time.Millisecond

func (kv *KVServer) isDuplicatedRequest(clientId, commandId int64) bool {
	lastReply, ok := kv.lastOperations[clientId]
	return ok && commandId <= lastReply.MaxAppliedCommandId
}

func (kv *KVServer) getNotifyChan(index int) chan *CommandReply {
	if _, ok := kv.notifyChans[index]; !ok {
		kv.notifyChans[index] = make(chan *CommandReply, 1)
	}
	return kv.notifyChans[index]
}

func (kv *KVServer) removeOutdatedNotifyChan(index int) {
	delete(kv.notifyChans, index)
}

func (kv *KVServer) applyToStateMachine(command Command) *CommandReply {
	var value string
	var err Err
	switch command.Op {
	case OP_PUT:
		err = kv.stateMachine.Put(command.Key, command.Value)
	case OP_APPEND:
		err = kv.stateMachine.Append(command.Key, command.Value)
	case OP_GET:
		value, err = kv.stateMachine.Get(command.Key)
	}
	return &CommandReply{err, value}
}

func (kv *KVServer) needSnapshot() bool {
	return kv.maxraftstate != -1 && kv.rf.GetRaftStateSize() >= kv.maxraftstate
}

func (kv *KVServer) taskSnapshot(index int) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.stateMachine)
	e.Encode(kv.lastOperations)
	kv.rf.Snapshot(index, w.Bytes())
}

func (kv *KVServer) restoreSnapshot(snapshot []byte) {
	if snapshot == nil || len(snapshot) == 0 {
		return
	}
	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)
	var stateMachine MemoryKV
	var lastOperations map[int64]OperationInfo
	if d.Decode(&stateMachine) != nil || d.Decode(&lastOperations) != nil {
		DPrintf("[Node %v] restores snapshot failed", kv.rf.Who())
	}
	kv.stateMachine = &stateMachine
	kv.lastOperations = lastOperations
}
