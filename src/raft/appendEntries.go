package raft

type Entry struct {
	Index   int         // log index
	Term    int         // log term
	Command interface{} // command
}

type AppendEntriesArgs struct {
	Term         int     // leader's term
	LeaderId     int     // so follower can redirect clients
	PrevLogIndex int     // index of log entry immediately preceding new ones
	PrevLogTerm  int     // term of prevLogindex entry
	Entries      []Entry // log entries to store(empty for heartbeat), may send more than one for efficiency
	LeaderCommit int     // leader's commit index
}

type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching prevLogIndex and prevLogTerm
}

// AppendEntries RPC
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	DPrintf("Node %d : term %d follower receive AE from %d", rf.me, rf.commitIndex, args.LeaderId)

	reply.Success = false
	reply.Term = rf.currentTerm

	// AE RPC 1
	if args.Term < rf.currentTerm {
		return
	}
	// all server rule 2
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = FOLLOWER
		rf.votedFor = -1
	}

	rf.electionTimer.Reset(RandomElectionTimeoutDuration())

	if rf.role == CANDIDATE {
		rf.role = FOLLOWER
	}
	// AE RPC rule 2
	// if rf.logs[len(rf.logs)-1].Index < args.PrevLogIndex {
	// 	// conflict
	// }
	reply.Success = true
}

func (rf *Raft) appendEntries(heartbeat bool) {
	// lastLog := rf.logs[len(rf.logs)-1]

	for peer := range rf.peers {
		if peer == rf.me {
			rf.electionTimer.Reset(RandomElectionTimeoutDuration())
			continue
		}
		if heartbeat {
			// nextIndex := rf.nextIndex[peer]
			// if nextIndex < 0 {
			// 	nextIndex = 1
			// }
			// if lastLog.Index + 1 < nextIndex {
			// 	nextIndex = lastLog.Index
			// }
			// prevLog := rf.logs[nextIndex - 1]
			args := AppendEntriesArgs{
				Term:     rf.currentTerm,
				LeaderId: rf.me,
			}
			reply := AppendEntriesReply{}
			go rf.sendAppendEntries(peer, &args, &reply)
		}
	}
}

func (rf *Raft) sendAppendEntries(severId int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[severId].Call("Raft.AppendEntries", args, reply)
	return ok
}
