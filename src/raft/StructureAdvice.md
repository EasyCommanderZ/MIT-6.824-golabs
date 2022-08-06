# Raft Structure Advice (From TA)

http://nil.csail.mit.edu/6.824/2020/labs/raft-structure.txt

- A Raft instance has to deal with the arrival of external events :
  - Start() calls
  - AppendEntries
  - RequestVote RPCs
  - RPC Replies
- It hash to execute periodic tasks:
  - elections
  - Heart-beats

----

- use time.Sleep in a loop to simulate timer instead of using time.Timer or time.Ticker
- Have a separate long-running goroutine that sends committed log entries in order on the applyCh. It must be separate since sending on the applyCh can block, and it must be single goroutine since otherwise it may be hard to ensure the log order; the code that advances commitIndex wil need to kick the apply goroutine, suggests using condition variable (sync.cond)
- Each RPC sent on its own goroutine
- Keep in mind the the RPC can be delayed, reordered and more.

----

### MOST IMPORTANT : STICK TO FIGURE 2 IN RAFT PAPER !