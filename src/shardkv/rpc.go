package shardkv

import "6.824/shardctrler"

type CommandArgs struct {
	Key       string
	Value     string
	OP        string
	ClientId  int64
	CommandId int64
}

type CommandReply struct {
	Err   Err
	Value string
}

const (
	OP_GET    = "OP_GET"
	OP_APPEND = "OP_APPEND"
	OP_PUT    = "OP_PUT"
)

const (
	SHD_OPERATION     = "OPERATION"
	SHD_Configuration = "CONFIGURATION"
	SHD_InsertShards  = "INSERTSHARDS"
	SHD_DeleteShards  = "DELETESHARDS"
	SHD_EmptyEntry    = "EMPTYENTRY"
)

type Command struct {
	OP   string
	Data interface{}
}

type OperationInfo struct {
	MaxAppliedCommandId int64
	LastReply           *CommandReply
}

func (info OperationInfo) dcopy() OperationInfo {
	return OperationInfo{
		MaxAppliedCommandId: info.MaxAppliedCommandId,
		LastReply: &CommandReply{
			Err:   info.LastReply.Err,
			Value: info.LastReply.Value,
		},
	}
}

type ShardOperationArgs struct {
	ConfigNum int
	ShardIds  []int
}

type ShardOperationReply struct {
	Err            Err
	ConfigNum      int
	Shards         map[int]map[string]string
	LastOperations map[int64]OperationInfo
}

func NewOperationCommand(args *CommandArgs) Command {
	return Command{
		OP:   SHD_OPERATION,
		Data: *args,
	}
}

func NewConfigurationCommand(config *shardctrler.Config) Command {
	return Command{
		OP:   SHD_Configuration,
		Data: *config,
	}
}

func NewInsertShardsCommand(reply *ShardOperationReply) Command {
	return Command{
		OP:   SHD_InsertShards,
		Data: *reply,
	}
}

func NewDeleteShardsCommand(args *ShardOperationArgs) Command {
	return Command{
		OP:   SHD_DeleteShards,
		Data: *args,
	}
}

func NewEmptyEntryCommand() Command {
	return Command{
		OP:   SHD_EmptyEntry,
		Data: nil,
	}
}
