package shardkv

import (
	"bytes"

	"6.824/labgob"
	"6.824/shardctrler"
)

func (kv *ShardKV) restoreSnapshot(snapshot []byte) {
	if snapshot == nil || len(snapshot) == 0 {
		kv.initStateMachines()
		return
	}
	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)
	var stateMachines map[int]*Shard
	var lastOperations map[int64]OperationInfo
	var currentConfig shardctrler.Config
	var lastConfig shardctrler.Config
	if d.Decode(&stateMachines) != nil ||
		d.Decode(&lastOperations) != nil ||
		d.Decode(&currentConfig) != nil ||
		d.Decode(&lastConfig) != nil {
	}
	kv.stateMachines, kv.lastOperations, kv.currentConfig, kv.lastConfig = stateMachines, lastOperations, currentConfig, lastConfig
}

func (kv *ShardKV) needSnapshot() bool {
	return kv.maxraftstate != -1 && kv.rf.GetRaftStateSize() >= kv.maxraftstate
}

func (kv *ShardKV) takeSnapshot(index int) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.stateMachines)
	e.Encode(kv.lastOperations)
	e.Encode(kv.currentConfig)
	e.Encode(kv.lastConfig)
	kv.rf.Snapshot(index, w.Bytes())
}
