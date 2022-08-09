package raft

func (rf *Raft) apply() {
	DPrintf("Node %v apply", rf.me)
	rf.applyCond.Broadcast()
}

func (rf *Raft) applier() {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.LastApplied >= rf.commitIndex {
			rf.applyCond.Wait()
		}

		firstLogIndex, commitIndex, lastApplied := rf.getFirstLog().Index, rf.commitIndex, rf.LastApplied
		entries := make([]Entry, commitIndex-lastApplied)
		copy(entries, rf.logs[lastApplied-firstLogIndex+1:commitIndex-firstLogIndex+1])
		rf.mu.Unlock()

		for _, entry := range entries {
			rf.applyCh <- ApplyMsg{
				CommandValid: true,
				Command:      entry.Command,
				CommandIndex: entry.Index,
			}
		}
		rf.mu.Lock()
		rf.LastApplied = max(rf.LastApplied, commitIndex)
		rf.mu.Unlock()
	}
}
