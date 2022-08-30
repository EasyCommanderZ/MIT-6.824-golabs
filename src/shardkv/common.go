package shardkv

import (
	"sync"
	"time"

	"6.824/raft"
	"6.824/shardctrler"
)

//
// Sharded key/value server.
// Lots of replica groups, each running Raft.
// Shardctrler decides which group serves each shard.
// Shardctrler may change shard assignment from time to time.
//
// You will have to modify these definitions.
//

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongGroup  = "ErrWrongGroup"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeout     = "ErrTimeout"
	ErrNotReady    = "ErrNotReady"
	ErrOutDated    = "ErrOutDated"
)

const (
	ExecuteTimeout            = 500 * time.Millisecond
	ConfigureTimeout          = 100 * time.Microsecond
	MigrationTimeout          = 50 * time.Microsecond
	GCTimeout                 = 50 * time.Microsecond
	EmptyEntryDetectorTimeout = 200 * time.Microsecond
)

type Err string

// Put or Append
type PutAppendArgs struct {
	// You'll have to add definitions here.
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

func (kv *ShardKV) initStateMachines() {
	for i := 0; i < shardctrler.NShards; i++ {
		if _, ok := kv.stateMachines[i]; !ok {
			kv.stateMachines[i] = NewShard()
		}
	}
}

func (kv *ShardKV) isDuplicateRequest(clientId int64, commandId int64) bool {
	opInfo, ok := kv.lastOperations[clientId]
	return ok && opInfo.MaxAppliedCommandId >= commandId
}

func (kv *ShardKV) getNotifyChans(index int) chan *CommandReply {
	if _, ok := kv.notifyChans[index]; !ok {
		kv.notifyChans[index] = make(chan *CommandReply, 1)
	}
	return kv.notifyChans[index]
}

func (kv *ShardKV) removeOutDatedNotifyChan(index int) {
	delete(kv.notifyChans, index)
}

func (kv *ShardKV) canServer(shardId int) bool {
	return kv.currentConfig.Shards[shardId] == kv.gid && (kv.stateMachines[shardId].Status == STATUS_SERVING || kv.stateMachines[shardId].Status == STATUS_GC)
}

func (kv *ShardKV) Execute(command Command, reply *CommandReply) {
	index, _, isLeader := kv.rf.Start(command)
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	kv.mu.Lock()
	ch := kv.getNotifyChans(index)
	kv.mu.Unlock()

	select {
	case result := <-ch:
		reply.Value, reply.Err = result.Value, result.Err
	case <-time.After(ExecuteTimeout):
		reply.Err = ErrTimeout
	}

	go func() {
		kv.mu.Lock()
		kv.removeOutDatedNotifyChan(index)
		kv.mu.Unlock()
	}()
}

// update to latest configuration
func (kv *ShardKV) configurationAction() {
	canGetNextConfig := true
	kv.mu.Lock()

	for _, shard := range kv.stateMachines {
		if shard.Status != STATUS_SERVING {
			canGetNextConfig = false
			break
		}
	}

	currentConfigNum := kv.currentConfig.Num
	kv.mu.Unlock()
	if canGetNextConfig {
		nextConfig := kv.sc.Query(currentConfigNum + 1)
		if nextConfig.Num == currentConfigNum+1 {
			kv.Execute(NewConfigurationCommand(&nextConfig), &CommandReply{})
		}
	}
}

func (kv *ShardKV) getShardIdsByStatus(status string) map[int][]int {
	gid2shardIds := make(map[int][]int)
	for i, shard := range kv.stateMachines {
		if shard.Status == status {
			gid := kv.lastConfig.Shards[i]
			if gid != 0 {
				if _, ok := gid2shardIds[gid]; !ok {
					gid2shardIds[gid] = make([]int, 0)
				}
				gid2shardIds[gid] = append(gid2shardIds[gid], i)
			}
		}
	}
	return gid2shardIds
}

// migrate related shards
// pull to migrate
func (kv *ShardKV) migrationAction() {
	kv.mu.Lock()
	gid2shardIds := kv.getShardIdsByStatus(STATUS_PULLING)
	var wg sync.WaitGroup
	for gid, shardIds := range gid2shardIds {
		wg.Add(1)
		go func(servers []string, configNum int, shardIds []int) {
			defer wg.Done()
			pullTaskArgs := ShardOperationArgs{
				ConfigNum: configNum,
				ShardIds:  shardIds,
			}
			for _, server := range servers {
				var pullTaskReply ShardOperationReply
				serverEnd := kv.make_end(server)
				if serverEnd.Call("ShardKV.GetShardsData", &pullTaskArgs, &pullTaskReply) && pullTaskReply.Err == OK {
					kv.Execute(NewInsertShardsCommand(&pullTaskReply), &CommandReply{})
				}
			}
		}(kv.lastConfig.Groups[gid], kv.currentConfig.Num, shardIds)
	}
	kv.mu.Unlock()
	wg.Wait()
}

func (kv *ShardKV) GetShardsData(args *ShardOperationArgs, reply *ShardOperationReply) {
	if _, isLeader := kv.rf.GetState(); !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.currentConfig.Num < args.ConfigNum {
		reply.Err = ErrNotReady
		return
	}

	reply.Shards = make(map[int]map[string]string)
	for _, shardId := range args.ShardIds {
		reply.Shards[shardId] = kv.stateMachines[shardId].dcopy()
	}
	reply.LastOperations = make(map[int64]OperationInfo)
	for clientId, operation := range kv.lastOperations {
		reply.LastOperations[clientId] = operation.dcopy()
	}
	reply.ConfigNum, reply.Err = args.ConfigNum, OK
}

func (kv *ShardKV) DeleteShardsData(args *ShardOperationArgs, reply *ShardOperationReply) {
	if _, isLeader := kv.rf.GetState(); !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	kv.mu.Lock()
	if kv.currentConfig.Num < args.ConfigNum {
		reply.Err = OK
		kv.mu.Unlock()
		return
	}
	kv.mu.Unlock()
	var commandReply CommandReply
	kv.Execute(NewDeleteShardsCommand(args), &commandReply)
	reply.Err = commandReply.Err
}

func (kv *ShardKV) gcAction() {
	kv.mu.Lock()
	gid2shardIds := kv.getShardIdsByStatus(STATUS_GC)
	var wg sync.WaitGroup
	for gid, shardIds := range gid2shardIds {
		wg.Add(1)
		go func(servers []string, configNum int, shardIds []int) {
			defer wg.Done()
			gcTaskArgs := ShardOperationArgs{
				ConfigNum: configNum,
				ShardIds:  shardIds,
			}
			for _, server := range servers {
				var gcTaskReply ShardOperationReply
				serverEnd := kv.make_end(server)
				if serverEnd.Call("ShardKV.DeleteShardsData", &gcTaskArgs, &gcTaskReply) && gcTaskReply.Err == OK {
					kv.Execute(NewDeleteShardsCommand(&gcTaskArgs), &CommandReply{})
				}
			}
		}(kv.lastConfig.Groups[gid], kv.currentConfig.Num, shardIds)
	}
	kv.mu.Unlock()
	wg.Wait()
}

func (kv *ShardKV) checkEntryInCurrentTermAction() {
	if !kv.rf.HasLogInCurrentTerm() {
		kv.Execute(NewEmptyEntryCommand(), &CommandReply{})
	}
}

func (kv *ShardKV) applyLogToStateMachines(operation *CommandArgs, shardId int) *CommandReply {
	var value string
	var err Err
	switch operation.OP {
	case OP_PUT:
		err = kv.stateMachines[shardId].Put(operation.Key, operation.Value)
	case OP_APPEND:
		err = kv.stateMachines[shardId].Append(operation.Key, operation.Value)
	case OP_GET:
		value, err = kv.stateMachines[shardId].Get(operation.Key)
	}
	return &CommandReply{
		Value: value,
		Err:   err,
	}
}

func (kv *ShardKV) applyOperation(msg *raft.ApplyMsg, operation *CommandArgs) *CommandReply {
	var reply *CommandReply
	shardId := key2shard(operation.Key)
	if kv.canServer(shardId) {
		if operation.OP != OP_GET && kv.isDuplicateRequest(operation.ClientId, operation.CommandId) {
			return kv.lastOperations[operation.ClientId].LastReply
		} else {
			reply = kv.applyLogToStateMachines(operation, shardId)
			if operation.OP != OP_GET {
				kv.lastOperations[operation.ClientId] = OperationInfo{
					MaxAppliedCommandId: operation.CommandId,
					LastReply:           reply,
				}
			}
			return reply
		}
	}
	return &CommandReply{
		Err:   ErrWrongGroup,
		Value: "",
	}
}

func (kv *ShardKV) updateShardStatus(nextConfig *shardctrler.Config) {
	for i := 0; i < shardctrler.NShards; i++ {
		// pull data
		if kv.currentConfig.Shards[i] != kv.gid && nextConfig.Shards[i] == kv.gid {
			// need pull data
			gid := kv.currentConfig.Shards[i]
			if gid != 0 {
				kv.stateMachines[i].Status = STATUS_PULLING
			}
		}
		if kv.currentConfig.Shards[i] == kv.gid && nextConfig.Shards[i] != kv.gid {
			gid := nextConfig.Shards[i]
			if gid != 0 {
				kv.stateMachines[i].Status = STATUS_BEPULLING
			}
		}
	}
}

func (kv *ShardKV) applyConfiguration(nextConfig *shardctrler.Config) *CommandReply {
	if nextConfig.Num == kv.currentConfig.Num+1 {
		kv.updateShardStatus(nextConfig)
		kv.lastConfig = kv.currentConfig
		kv.currentConfig = *nextConfig
		return &CommandReply{
			Err:   OK,
			Value: "",
		}
	}
	return &CommandReply{
		Err:   ErrOutDated,
		Value: "",
	}
}

func (kv *ShardKV) applyInsertShards(shardsInfo *ShardOperationReply) *CommandReply {
	if shardsInfo.ConfigNum == kv.currentConfig.Num {
		for shardId, shardData := range shardsInfo.Shards {
			shard := kv.stateMachines[shardId]
			if shard.Status == STATUS_PULLING {
				for key, value := range shardData {
					shard.KV[key] = value
				}
				shard.Status = STATUS_GC
			} else {
				break
			}
		}
		for clientId, operationInfo := range shardsInfo.LastOperations {
			if lastOperation, ok := kv.lastOperations[clientId]; !ok || lastOperation.MaxAppliedCommandId < operationInfo.MaxAppliedCommandId {
				kv.lastOperations[clientId] = operationInfo
			}
		}
		return &CommandReply{
			Err:   OK,
			Value: "",
		}
	}
	return &CommandReply{Err: ErrOutDated}
}

func (kv *ShardKV) applyDeleteShards(shardsInfo *ShardOperationArgs) *CommandReply {
	if shardsInfo.ConfigNum == kv.currentConfig.Num {
		for _, shardId := range shardsInfo.ShardIds {
			shard := kv.stateMachines[shardId]
			if shard.Status == STATUS_GC {
				shard.Status = STATUS_SERVING
			} else if shard.Status == STATUS_BEPULLING {
				kv.stateMachines[shardId] = NewShard()
			} else {
				break
			}
		}
		return &CommandReply{Err: OK}
	}
	return &CommandReply{Err: OK}
}

func (kv *ShardKV) applyEmptyEntry() *CommandReply {
	return &CommandReply{Err: OK}
}
