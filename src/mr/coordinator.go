package mr

import (
	"log"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

const maxTaskTime = 10 // max task running seconds, if exceed then re-assign

type Coordinator struct {
	// Your definitions here.
	nMap    int
	nReduce int

	taskQueue    chan TaskInfo
	queueEmpty   bool
	taskTraceMap map[string]TaskInfo

	Phase string
	lck   sync.Mutex
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (c *Coordinator) AssignTask(args *AssignTaskArgs, reply *AssignTaskReply) error {
	// generate task info. all type of task share the same format
	c.lck.Lock()
	if c.Phase == DONE {
		task := TaskInfo{
			TaskType: QUIT,
		}
		c.taskQueue <- task
		log.Println("Queue a quit job")
	}
	c.lck.Unlock()

	task := <-c.taskQueue

	task.Deadline = time.Now().Add(maxTaskTime * time.Second)
	reply.Info = task
	reply.NMap = c.nMap
	reply.NReduce = c.nReduce

	c.lck.Lock()
	defer c.lck.Unlock()
	if c.Phase != DONE {
		key := c.Phase + "_" + strconv.Itoa(task.Id)
		c.taskTraceMap[key] = task
	}

	return nil
}

func (c *Coordinator) ReportTask(args *ReportTaskArgs, reply *ReportTaskReply) error {
	key := args.TaskType + "_" + strconv.Itoa(args.TaskID)
	c.lck.Lock()
	// defer c.lck.Unlock()
	// if task not timeout, accept; else reject
	// if accept, remove task trace
	if time.Now().After(c.taskTraceMap[key].Deadline) {
		reply.Accept = false
		// timeout task will be retrived automatically
	} else {
		// process task result
		log.Printf("%s task %d accepted, process intermediate file\n", args.TaskType, args.TaskID)
		if args.TaskType == MAP {
			for i := 0; i < c.nReduce; i++ {
				tmpfilename, finalfilename := tmpMapOutFile(args.TaskID, i), finalMapOutFile(args.TaskID, i)
				err := os.Rename(tmpfilename, finalfilename)
				if err != nil {
					log.Fatalf("Failed to validate temp file : %s to %s", tmpfilename, finalfilename)
				}
			}
		} else if args.TaskType == REDUCE {
			tmpfilename, finalfilename := tmpReduceOutFile(args.TaskID), finalReduceOutFile(args.TaskID)
			err := os.Rename(tmpfilename, finalfilename)
			if err != nil {
				log.Fatalf("Failed to validate temp file : %s to %s", tmpfilename, finalfilename)
			}
		}

		delete(c.taskTraceMap, key)
		reply.Accept = true
	}

	// c.checkPhase()
	// checkpoint()
	// println(len(c.taskQueue), len(c.taskTraceMap))
	oldPhase := c.Phase
	if len(c.taskQueue) == 0 && len(c.taskTraceMap) == 0 && c.Phase != DONE {
		if c.Phase == MAP {
			c.Phase = REDUCE
			go c.generateReduceTasks()
		} else if c.Phase == REDUCE {
			c.Phase = DONE
		}
		log.Printf("Coordinator phase change from %s to %s \n", oldPhase, c.Phase)
	}

	c.lck.Unlock()
	return nil
}

func (c *Coordinator) generateReduceTasks() {
	log.Printf("Generating REDUCE tasks, total : %d\n", c.nReduce)
	for i := 0; i < c.nReduce; i++ {
		task := TaskInfo{
			Id:       i,
			TaskType: REDUCE,
			// deadline will be updated when task is assigned
		}
		// add to traceMap when task is assigned
		c.taskQueue <- task
	}
	log.Println("REDUCE tasks generated")
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.
	c.lck.Lock()
	if c.Phase == DONE {
		ret = true
	}
	c.lck.Unlock()
	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	log.SetPrefix("[Coordinator] ")
	log.Println("MakeCoordinator")

	c := Coordinator{
		Phase:        MAP,
		nMap:         len(files),
		nReduce:      nReduce,
		taskQueue:    make(chan TaskInfo, int(math.Max(float64(nReduce), float64(len(files))))),
		queueEmpty:   false,
		taskTraceMap: make(map[string]TaskInfo),
	}

	// Your code here.
	log.Printf("Generating MAP tasks, total : %d\n", c.nMap)
	for i, file := range files {
		task := TaskInfo{
			Id:       i,
			TaskType: MAP,
			Filename: file,
			// deadline will be updated when task is assigned
		}
		// add to traceMap when task is assigned
		c.taskQueue <- task
	}
	log.Println("MAP tasks generated, coordinator starting")

	c.server()
	// retrive timeout tasks
	go c.retriveTimeoutTasks()

	return &c
}

func (c *Coordinator) retriveTimeoutTasks() {
	for {
		time.Sleep(500 * time.Millisecond)
		c.lck.Lock()
		// blocked here for lock
		// checkpoint()
		// println(len(c.taskTraceMap))
		for _, task := range c.taskTraceMap {
			// quit job will not be retrived
			if time.Now().After(task.Deadline) {
				log.Printf("%s Task %d timeout, retrive & re-queue\n", task.TaskType, task.Id)
				delete(c.taskTraceMap, task.TaskType+"_"+strconv.Itoa(task.Id))
				c.taskQueue <- task
			}
		}
		c.lck.Unlock()
	}
}
