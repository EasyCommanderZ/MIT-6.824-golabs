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
type AssignTaskArgs struct {
	// LastTaskId   int
	// LastTaskType int
}

type AssignTaskReply struct {
	Info    TaskInfo
	NMap    int
	NReduce int
}

type ReportTaskArgs struct {
	TaskID   int
	TaskType string
}

type ReportTaskReply struct {
	Accept bool
}

const (
	MAP      = "MAP"
	REDUCE   = "REDUCE"
	FINISHED = "FINISHED"
	QUIT     = "QUIT"
	DONE     = "DONE"
)

type TaskInfo struct {
	Id       int
	TaskType string
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
