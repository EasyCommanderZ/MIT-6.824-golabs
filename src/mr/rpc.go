package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
	"time"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
type getTaskArgs struct {
	LastTaskId   int
	LastTaskType int
}

type getTaskReply struct {
	info    TaskInfo
	nMap    int
	nReduce int
}

type reportTaskArgs struct {
	taskID int
}

type reportTaskReply struct {
	Accept bool
}

const (
	mapTask    = 0
	reduceTask = 1
)

type TaskInfo struct {
	Id       int
	TaskType int
	Filename string
	Deadline time.Time
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
