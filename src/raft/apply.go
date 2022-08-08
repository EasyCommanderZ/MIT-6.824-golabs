package raft

func (rf *Raft) apply() {
	DPrintf("Node %v apply", rf.me)
	rf.applyCond.Broadcast()
}

func (rf *Raft) applier() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for !rf.killed() {
		if rf.commitIndex > rf.LastApplied && rf.getLastLog().Index > rf.LastApplied {
			rf.LastApplied++
			appmsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[rf.LastApplied].Command,
				CommandIndex: rf.LastApplied,
			}
			rf.mu.Unlock()
			rf.applyCh <- appmsg
			rf.mu.Lock()
		} else {
			rf.applyCond.Wait()
		}
	}
}
