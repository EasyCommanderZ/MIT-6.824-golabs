package raft

func (rf *Raft) StartElection() {
	DPrintf("[Node %v] starts election in term %v", rf.me, rf.currentTerm)
	rf.role = CANDIDATE
	rf.currentTerm += 1
	rf.votedFor = rf.me
	args := RequestVoteArgs{
		// Lab 2A
		Term:        rf.currentTerm,
		CandidateId: rf.me,
	}
	grantedVote := 1

	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}
		reply := RequestVoteReply{}
		go func(peer int) {
			if rf.sendRequestVote(peer, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				DPrintf("Node %v receive RequestVoteReply from %v. self term : %v, reply term: %v", rf.me, peer, rf.currentTerm, reply.Term)
				// term may updated already
				if rf.currentTerm == args.Term && rf.role == CANDIDATE {
					if reply.VoteGranted {
						grantedVote += 1
						if grantedVote > len(rf.peers)/2 {
							// win the election
							DPrintf("Node %v win the election in term %v", rf.me, rf.currentTerm)
							rf.role = LEADER
							rf.appendEntries(true)
						}
					} else if reply.Term > rf.currentTerm {
						DPrintf("Node %v find higher term from node %v", rf.me, peer)
						rf.role = FOLLOWER
						rf.currentTerm = reply.Term
						rf.votedFor = -1
						rf.persist()
					}
				}
			}
		}(peer)
	}
}
