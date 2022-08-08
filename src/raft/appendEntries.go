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
	Term     int  // currentTerm, for leader to update itself
	Success  bool // true if follower contained entry matching prevLogIndex and prevLogTerm
	Conflict bool
	XTerm    int // XTerm：Follower中与Leader冲突的Log的Term号
	XIndex   int // XIndex：Follower中，对应XTerm的第一条Log 条目的槽位
	XLen     int // Xlen ：日志长度。如果Follower在对应位置没有Log，那么Xterm会返回-1。表示空白的Log槽位数
}

// AppendEntries RPC
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// defer rf.persist()
	DPrintf("Node %d : term %d follower receive AE from %d, args: %v", rf.me, rf.currentTerm, args.LeaderId, args)

	reply.Success = false
	reply.Term = rf.currentTerm

	// AE RPC 1
	if args.Term < rf.currentTerm {
		return
	}
	// all server rule 2
	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
		return
	}

	rf.electionTimer.Reset(RandomElectionTimeoutDuration())

	// candidate rule 3
	if rf.role == CANDIDATE {
		rf.role = FOLLOWER
	}
	// AE RPC rule 2
	// Reply false if log doesn't contain an entry at preLogIndex whose term matches prevLogterm
	if rf.getLastLog().Index < args.PrevLogIndex {
		// conflict
		// last log index < prevLogIndex -> shorter
		reply.Conflict = true
		reply.XTerm = -1
		reply.XIndex = -1
		reply.XLen = len(rf.logs)
		return
	}
	if rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		// term not match, find the earliest one, roll back
		reply.Conflict = true
		xTerm := rf.logs[args.PrevLogIndex].Term
		for xIndex := args.PrevLogIndex; xIndex > 0; xIndex-- {
			if rf.logs[xIndex-1].Term != xTerm {
				reply.XIndex = xIndex
				break
			}
		}
		reply.XTerm = xTerm
		reply.XLen = len(rf.logs)
		return
	}

	for index, entry := range args.Entries {
		// append entries rule 3
		// If an existing entry conflicts with a new one (same index but different terms), delete the existing entry and all that follow it
		if entry.Index <= rf.getLastLog().Index && rf.logs[entry.Index].Term != entry.Term {
			rf.logs = rf.logs[:entry.Index]
			rf.persist()
		}

		// append entries rule 4
		// Append any new entries not already in the log
		if entry.Index > rf.getLastLog().Index {
			rf.logs = append(rf.logs, args.Entries[index:]...)
			rf.persist()
			break
		}
	}

	// append entries rule 5
	// if leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.getLastLog().Index)
		rf.apply()
	}
	reply.Success = true
}

func (rf *Raft) appendEntries(heartbeat bool) {
	lastLog := rf.getLastLog()

	for peer := range rf.peers {
		if peer == rf.me {
			rf.electionTimer.Reset(RandomElectionTimeoutDuration())
			continue
		}
		// leader rule 3
		if heartbeat || lastLog.Index >= rf.nextIndex[peer] {
			nextIndex := rf.nextIndex[peer]
			if nextIndex <= 0 {
				nextIndex = 1
			}
			if lastLog.Index+1 < nextIndex {
				nextIndex = lastLog.Index
			}
			prevLog := rf.logs[nextIndex-1]
			args := AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLog.Index,
				PrevLogTerm:  prevLog.Term,
				Entries:      make([]Entry, lastLog.Index-nextIndex+1),
				LeaderCommit: rf.commitIndex,
			}
			DPrintf("%v to %v, args: %v", rf.me, peer, args)
			copy(args.Entries, rf.logs[nextIndex:])
			go rf.sendAppendEntryAndUpdate(peer, &args)
		}
	}
}

func (rf *Raft) sendAppendEntryAndUpdate(serverId int, args *AppendEntriesArgs) {
	var reply AppendEntriesReply
	ok := rf.sendAppendEntries(serverId, args, &reply)
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.Term > rf.currentTerm {
		rf.setNewTerm(reply.Term)
		return
	}
	if args.Term == rf.currentTerm {
		// still in this term

		// rules for leader 3.1
		// if successfule: update nextIndex and matchIndex for follower
		if reply.Success {
			match := args.PrevLogIndex + len(args.Entries)
			next := match + 1
			rf.nextIndex[serverId] = max(rf.nextIndex[serverId], next)
			rf.matchIndex[serverId] = max(rf.matchIndex[serverId], match)
			DPrintf("reply from %v success, update nextIndex to %v, matchIndex to %v", serverId, rf.nextIndex[serverId], rf.matchIndex[serverId])
		} else if reply.Conflict {
			// if AppendEntries fails because of log inconsistency : decrement nextIndex and retry
			if reply.XTerm == -1 {
				// shorter logs
				rf.nextIndex[serverId] = reply.XLen
			} else {
				lastLogInXTerm := rf.findLastLogInXTerm(reply.XTerm)
				if lastLogInXTerm > 0 {
					rf.nextIndex[serverId] = lastLogInXTerm
				} else {
					rf.nextIndex[serverId] = reply.XIndex
				}
			}
		} else if rf.nextIndex[serverId] > 1 {
			rf.nextIndex[serverId]--
		}
		rf.leaderCommitLog()
	}
}

func (rf *Raft) leaderCommitLog() {
	// leader rule 4
	// if there exists an N such that N > commitIndex, a mojority of mattchIndex[i] >= N, and log[N].term == currentTerm: set commitIndex = N
	if rf.role != LEADER {
		return
	}
	DPrintf("leader commit log: commitIndex: %v, lastLogIndex: %v", rf.commitIndex, rf.getLastLog().Index)
	for i := rf.commitIndex + 1; i <= rf.getLastLog().Index; i++ {
		if rf.logs[i].Term != rf.currentTerm {
			continue
		}
		cnt := 1
		for serverId := 0; serverId < len(rf.peers); serverId++ {
			if serverId != rf.me && rf.matchIndex[serverId] >= i {
				cnt++
			}
			if cnt > len(rf.peers)/2 {
				rf.commitIndex = i
				rf.apply()
				break
			}
		}
	}
}

func (rf *Raft) findLastLogInXTerm(term int) int {
	for i := rf.logs[len(rf.logs)-1].Index; i > 0; i-- {
		curTerm := rf.logs[i].Term
		if curTerm == term {
			return i
		} else if curTerm < term {
			break
		}
	}
	return -1
}

func (rf *Raft) sendAppendEntries(severId int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[severId].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) setNewTerm(term int) {
	rf.currentTerm = term
	rf.role = FOLLOWER
	rf.votedFor = -1
	rf.persist()
}

func (rf *Raft) getLastLog() *Entry {
	return &rf.logs[len(rf.logs)-1]
}
