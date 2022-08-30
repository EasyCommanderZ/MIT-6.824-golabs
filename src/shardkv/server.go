package shardkv

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	"6.824/shardctrler"
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type ShardKV struct {
	mu           sync.Mutex
	me           int
	rf           *raft.Raft
	applyCh      chan raft.ApplyMsg
	make_end     func(string) *labrpc.ClientEnd
	gid          int
	ctrlers      []*labrpc.ClientEnd
	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	sc          *shardctrler.Clerk
	lastApplied int
	dead        int32

	lastConfig    shardctrler.Config
	currentConfig shardctrler.Config

	stateMachines  map[int]*Shard
	lastOperations map[int64]OperationInfo
	notifyChans    map[int]chan *CommandReply
}

func (kv *ShardKV) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
}

func (kv *ShardKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
}

// the tester calls Kill() when a ShardKV instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (kv *ShardKV) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *ShardKV) killed() bool {
	return atomic.LoadInt32(&kv.dead) == 1
}

// servers[] contains the ports of the servers in this group.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
//
// the k/v server should snapshot when Raft's saved state exceeds
// maxraftstate bytes, in order to allow Raft to garbage-collect its
// log. if maxraftstate is -1, you don't need to snapshot.
//
// gid is this group's GID, for interacting with the shardctrler.
//
// pass ctrlers[] to shardctrler.MakeClerk() so you can send
// RPCs to the shardctrler.
//
// make_end(servername) turns a server name from a
// Config.Groups[gid][i] into a labrpc.ClientEnd on which you can
// send RPCs. You'll need this to send RPCs to other groups.
//
// look at client.go for examples of how to use ctrlers[]
// and make_end() to send RPCs to the group owning a specific shard.
//
// StartServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, make_end func(string) *labrpc.ClientEnd) *ShardKV {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	// labgob.Register(Op{})

	kv := new(ShardKV)
	kv.me = me
	kv.maxraftstate = maxraftstate
	kv.make_end = make_end
	kv.gid = gid
	kv.ctrlers = ctrlers
	// Your initialization code here.
	kv.dead = 0
	kv.lastApplied = 0
	kv.lastConfig = shardctrler.DefaultConfig()
	kv.currentConfig = shardctrler.DefaultConfig()
	labgob.Register(Command{})
	labgob.Register(ShardOperationArgs{})
	labgob.Register(ShardOperationReply{})
	labgob.Register(shardctrler.Config{})
	labgob.Register(CommandArgs{})

	// Use something like this to talk to the shardctrler:
	// kv.mck = shardctrler.MakeClerk(kv.ctrlers)
	kv.sc = shardctrler.MakeClerk(kv.ctrlers)
	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)
	kv.notifyChans = make(map[int]chan *CommandReply)
	kv.stateMachines = make(map[int]*Shard)
	kv.lastOperations = make(map[int64]OperationInfo)

	kv.restoreSnapshot(persister.ReadSnapshot())
	// apply committed to stateMachine
	go kv.applier()

	go kv.Serve(kv.configurationAction, ConfigureTimeout)
	go kv.Serve(kv.migrationAction, MigrationTimeout)
	go kv.Serve(kv.gcAction, GCTimeout)
	go kv.Serve(kv.checkEntryInCurrentTermAction, EmptyEntryDetectorTimeout)

	return kv
}

func (kv *ShardKV) Command(args *CommandArgs, reply *CommandReply) {
	kv.mu.Lock()
	if args.OP != OP_GET && kv.isDuplicateRequest(args.ClientId, args.CommandId) {
		lastReply := kv.lastOperations[args.ClientId].LastReply
		reply.Value, reply.Err = lastReply.Value, lastReply.Err
		kv.mu.Unlock()
		return
	}

	if !kv.canServer(key2shard(args.Key)) {
		reply.Err = ErrWrongGroup
		kv.mu.Unlock()
		return
	}

	kv.mu.Unlock()
	kv.Execute(NewOperationCommand(args), reply)
}

func (kv *ShardKV) Serve(action func(), timeout time.Duration) {
	for !kv.killed() {
		if _, isLeader := kv.rf.GetState(); isLeader {
			action()
		}
		time.Sleep(timeout)
	}
}

func (kv *ShardKV) applier() {
	for !kv.killed() {
		select {
		case msg := <-kv.applyCh:
			if msg.CommandValid {
				kv.mu.Lock()
				if msg.CommandIndex <= kv.lastApplied {
					kv.mu.Unlock()
					continue
				}
				kv.lastApplied = msg.CommandIndex
				var reply *CommandReply
				cmd := msg.Command.(Command)
				switch cmd.OP {
				case SHD_OPERATION:
					operation := cmd.Data.(CommandArgs)
					reply = kv.applyOperation(&msg, &operation)
				case SHD_Configuration:
					nextConfig := cmd.Data.(shardctrler.Config)
					reply = kv.applyConfiguration(&nextConfig)
				case SHD_InsertShards:
					shardsInfo := cmd.Data.(ShardOperationReply)
					reply = kv.applyInsertShards(&shardsInfo)
				case SHD_DeleteShards:
					shardsInfo := cmd.Data.(ShardOperationArgs)
					reply = kv.applyDeleteShards(&shardsInfo)
				case SHD_EmptyEntry:
					reply = kv.applyEmptyEntry()
				}

				if currentTerm, isLeader := kv.rf.GetState(); isLeader && msg.CommandTerm == currentTerm {
					ch := kv.getNotifyChans(msg.CommandIndex)
					ch <- reply
				}

				needsnapshot := kv.needSnapshot()
				if needsnapshot {
					kv.takeSnapshot(msg.CommandIndex)
				}
				kv.mu.Unlock()
			} else if msg.SnapshotValid {
				kv.mu.Lock()
				if kv.rf.CondInstallSnapshot(msg.SnapshotTerm, msg.SnapshotIndex, msg.Snapshot) {
					kv.restoreSnapshot(msg.Snapshot)
					kv.lastApplied = msg.SnapshotIndex
				}
				kv.mu.Unlock()
			} else {
				fmt.Println("WRONG STATE FOR APPLIER")
			}
		}
	}
}
