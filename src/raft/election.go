package raft

import "sync"

func (rf *Raft) StartElection() {
	DPrintf("[Node %v] starts election in term %v", rf.me, rf.currentTerm)
	rf.votedFor = rf.me
	lastLog := rf.getLastLog()
	args := RequestVoteArgs{
		// Lab 2A
		Term:        rf.currentTerm,
		CandidateId: rf.me,
		// Lab 2B
		LastLogIndex: lastLog.Index,
		LastLogTerm:  lastLog.Term,
	}
	grantedVote := 1
	var becomeLeader sync.Once
	rf.persist()

	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}
		// DPrintf("send requestVoteRPC")
		go func(peer int) {
			reply := RequestVoteReply{}
			if rf.sendRequestVote(peer, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				DPrintf("Node %v receive RequestVoteReply from %v. term : %v, reply : %v", rf.me, peer, rf.currentTerm, reply)
				if reply.Term > args.Term {
					rf.setNewTerm(args.Term)
					return
				}
				if reply.Term < args.Term {
					return
				}
				if !reply.VoteGranted {
					return
				}
				grantedVote++
				if grantedVote > len(rf.peers)/2 && rf.currentTerm == args.Term && rf.role == CANDIDATE {
					becomeLeader.Do(func() {
						DPrintf("%v win the election in term %v", rf.me, rf.currentTerm)
						rf.role = LEADER
						lastLogIndex := rf.getLastLog().Index
						for i, _ := range rf.peers {
							rf.nextIndex[i] = lastLogIndex + 1
							rf.matchIndex[i] = 0
						}
						rf.appendEntries(true)
						rf.heartbeatTimer.Reset(HeartbeatTimeoutDuration())
					})
				}
			}
		}(peer)
	}
}
