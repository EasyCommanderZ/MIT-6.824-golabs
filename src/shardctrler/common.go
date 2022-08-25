package shardctrler

import "time"

//
// Shard controler: assigns shards to replication groups.
//
// RPC interface:
// Join(servers) -- add a set of groups (gid -> server-list mapping).
// Leave(gids) -- delete a set of groups.
// Move(shard, gid) -- hand off one shard from current owner to gid.
// Query(num) -> fetch Config # num, or latest config if num==-1.
//
// A Config (configuration) describes a set of replica groups, and the
// replica group responsible for each shard. Configs are numbered. Config
// #0 is the initial configuration, with no groups and all shards
// assigned to group 0 (the invalid group).
//
// You will need to add fields to the RPC argument structs.
//

// The number of shards.
const NShards = 10
const ExecuteTimeout = 500 * time.Millisecond

// A configuration -- an assignment of shards to groups.
// Please don't change this.
type Config struct {
	Num    int              // config number
	Shards [NShards]int     // shard -> gid
	Groups map[int][]string // gid -> servers[]
}

const (
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeout     = "ErrTimeout"
)

type Err string

type JoinArgs struct {
	Servers map[int][]string // new GID -> servers mappings
}

type JoinReply struct {
	WrongLeader bool
	Err         Err
}

type LeaveArgs struct {
	GIDs []int
}

type LeaveReply struct {
	WrongLeader bool
	Err         Err
}

type MoveArgs struct {
	Shard int
	GID   int
}

type MoveReply struct {
	WrongLeader bool
	Err         Err
}

type QueryArgs struct {
	Num int // desired config number
}

type QueryReply struct {
	WrongLeader bool
	Err         Err
	Config      Config
}

const (
	OP_JOIN  = "OPJOIN"
	OP_LEAVE = "OPLEAVE"
	OP_MOVE  = "OPMOVE"
	OP_QUERY = "OPQUERY"
)

type CommandArgs struct {
	Servers   map[int][]string // Join
	GIDs      []int            // Leave
	Shard     int              // Move
	GID       int              // Move
	Num       int              // Query
	OP        string
	ClientId  int64
	CommandId int64
}
type CommandReply struct {
	Err    Err
	Config Config
}

type Command struct {
	*CommandArgs
}

type OperationInfo struct {
	MaxAppliedCommandId int64
	LastReply           *CommandReply
}

func (sc *ShardCtrler) isDuplicatedRequest(clientId int64, requestId int64) bool {
	opinfo, ok := sc.lastOperations[clientId]
	return ok && opinfo.MaxAppliedCommandId >= requestId
}

func (sc *ShardCtrler) removeOutdataedNotifyChan(index int) {
	delete(sc.notifyChans, index)
}

func (sc *ShardCtrler) getNotifyChan(index int) chan *CommandReply {
	if _, ok := sc.notifyChans[index]; !ok {
		sc.notifyChans[index] = make(chan *CommandReply, 1)
	}
	return sc.notifyChans[index]
}

func (sc *ShardCtrler) applyLogToStateMachine(command Command) *CommandReply {
	var config Config
	var err Err
	switch command.OP {
	case OP_JOIN:
		err = sc.stateMachine.Join(command.Servers)
	case OP_LEAVE:
		err = sc.stateMachine.Leave(command.GIDs)
	case OP_MOVE:
		err = sc.stateMachine.Move(command.Shard, command.GID)
	case OP_QUERY:
		config, err = sc.stateMachine.Query(command.Num)
	}
	return &CommandReply{err, config}
}

func dcopy(groups map[int][]string) map[int][]string {
	newGroups := make(map[int][]string)
	for gid, servers := range groups {
		newServers := make([]string, len(servers))
		copy(newServers, servers)
		newGroups[gid] = newServers
	}
	return newGroups
}

func DefaultConfig() Config {
	return Config{Groups: make(map[int][]string)}
}
