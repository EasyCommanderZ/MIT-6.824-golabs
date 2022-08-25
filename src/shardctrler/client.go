package shardctrler

//
// Shardctrler clerk.
//

import (
	"crypto/rand"
	"math/big"

	"6.824/labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
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
	// Your code here.
	ck.commandId = 0
	ck.clientId = nrand()
	ck.leaderId = 0
	return ck
}

func (ck *Clerk) Query(num int) Config {
	// args := &QueryArgs{}
	// // Your code here.
	// args.Num = num
	// for {
	// 	// try each known server.
	// 	for _, srv := range ck.servers {
	// 		var reply QueryReply
	// 		ok := srv.Call("ShardCtrler.Query", args, &reply)
	// 		if ok && reply.WrongLeader == false {
	// 			return reply.Config
	// 		}
	// 	}
	// 	time.Sleep(100 * time.Millisecond)
	// }
	return ck.Command(&CommandArgs{
		Num: num,
		OP:  OP_QUERY,
	})
}

func (ck *Clerk) Join(servers map[int][]string) {
	// args := &JoinArgs{}
	// // Your code here.
	// args.Servers = servers

	// for {
	// 	// try each known server.
	// 	for _, srv := range ck.servers {
	// 		var reply JoinReply
	// 		ok := srv.Call("ShardCtrler.Join", args, &reply)
	// 		if ok && reply.WrongLeader == false {
	// 			return
	// 		}
	// 	}
	// 	time.Sleep(100 * time.Millisecond)
	// }
	ck.Command(&CommandArgs{
		Servers: servers,
		OP:      OP_JOIN,
	})
}

func (ck *Clerk) Leave(gids []int) {
	// args := &LeaveArgs{}
	// // Your code here.
	// args.GIDs = gids

	// for {
	// 	// try each known server.
	// 	for _, srv := range ck.servers {
	// 		var reply LeaveReply
	// 		ok := srv.Call("ShardCtrler.Leave", args, &reply)
	// 		if ok && reply.WrongLeader == false {
	// 			return
	// 		}
	// 	}
	// 	time.Sleep(100 * time.Millisecond)
	// }
	ck.Command(&CommandArgs{
		GIDs: gids,
		OP:   OP_LEAVE,
	})
}

func (ck *Clerk) Move(shard int, gid int) {
	// args := &MoveArgs{}
	// // Your code here.
	// args.Shard = shard
	// args.GID = gid

	// for {
	// 	// try each known server.
	// 	for _, srv := range ck.servers {
	// 		var reply MoveReply
	// 		ok := srv.Call("ShardCtrler.Move", args, &reply)
	// 		if ok && reply.WrongLeader == false {
	// 			return
	// 		}
	// 	}
	// 	time.Sleep(100 * time.Millisecond)
	// }
	ck.Command(&CommandArgs{
		Shard: shard,
		GID:   gid,
		OP:    OP_MOVE,
	})
}

func (ck *Clerk) Command(args *CommandArgs) Config {
	args.ClientId, args.CommandId = ck.clientId, ck.commandId
	for {
		var reply CommandReply
		if !ck.servers[ck.leaderId].Call("ShardCtrler.Command", args, &reply) || reply.Err == ErrWrongLeader || reply.Err == ErrTimeout {
			ck.leaderId = (ck.leaderId + 1) % int64(len(ck.servers))
			continue
		}
		ck.commandId++
		return reply.Config
	}
}
