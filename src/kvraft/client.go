package kvraft

import (
	"crypto/rand"
	"math/big"

	"6.824/labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	leaderId  int64
	clientId  int64
	commandId int64
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.clientId = nrand()
	ck.leaderId = 0
	ck.commandId = 0

	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {

	// You will have to modify this function.
	value := ck.Command(&CommandArgs{
		Key: key,
		Op:  OP_GET,
	})
	return value
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//

// func (ck *Clerk) PutAppend(key string, value string, op string) {
// 	// You will have to modify this function.

// }

func (ck *Clerk) Put(key string, value string) {
	// ck.PutAppend(key, value, OP_PUT)
	ck.Command(&CommandArgs{
		Key:   key,
		Value: value,
		Op:    OP_PUT,
	})
}
func (ck *Clerk) Append(key string, value string) {
	// ck.PutAppend(key, value, APPEND)
	ck.Command(&CommandArgs{
		Key:   key,
		Value: value,
		Op:    OP_APPEND,
	})
}

func (ck *Clerk) Command(args *CommandArgs) string {
	args.ClientId = ck.clientId
	args.CommandId = ck.commandId
	for {
		reply := CommandReply{}
		if !ck.servers[ck.leaderId].Call("KVServer.Command", args, &reply) || reply.Err == ErrWrongLeader || reply.Err == ErrTimeOut {
			ck.leaderId = (ck.leaderId + 1) % int64(len(ck.servers))
			continue
		}
		ck.commandId++
		return reply.Value
	}
}
