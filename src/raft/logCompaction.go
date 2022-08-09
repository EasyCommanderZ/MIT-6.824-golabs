package raft

import (
	"bytes"

	"6.824/labgob"
)

func (rf *Raft) logCompact(logs []Entry) []Entry {
	return append([]Entry{}, logs...)
}

func (rf *Raft) encodeRaftState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	data := w.Bytes()
	return data
}

type InstallSnapshotArgs struct {
	Term              int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

type InstallSnapshotReply struct {
	Term      int
	Installed bool
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer DPrintf("{Node %v}'s state is {state %v,term %v,commitIndex %v,lastApplied %v,firstLog %v,lastLog %v} before processing InstallSnapshotRequest %v and reply InstallSnapshotResponse %v", rf.me, rf.role, rf.currentTerm, rf.commitIndex, rf.LastApplied, rf.getFirstLog(), rf.getLastLog(), args, reply)

	reply.Term = rf.currentTerm
	reply.Installed = false
	if args.Term < rf.currentTerm {
		return
	}

	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}
	rf.electionTimer.Reset(RandomElectionTimeoutDuration())

	// oudated snapshot
	if args.LastIncludedIndex <= rf.commitIndex {
		return
	}

	go func() {
		rf.applyCh <- ApplyMsg{
			SnapshotValid: true,
			Snapshot:      args.Data,
			SnapshotTerm:  args.LastIncludedTerm,
			SnapshotIndex: args.LastIncludedIndex,
		}
	}()
	reply.Installed = true
}

func (rf *Raft) sendInstallSnapshot(serverId int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[serverId].Call("Raft.InstallSnapshot", args, reply)
	return ok
}
