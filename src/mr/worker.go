package mr

import (
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	// Your worker implementation here.
	id := os.Getpid()
	log.SetPrefix("[Worker " + strconv.Itoa(id) + "] ")
	log.Printf("worker %d start working\n", id)

	for {
		// 1. request task from coordinator
		args, reply := AssignTaskArgs{}, AssignTaskReply{}
		ok := call("Coordinator.AssignTask", &args, &reply)
		if !ok {
			log.Println("RPC failed, re-apply")
			continue
		}
		log.Printf("Received %s task %d from Coordinator\n", reply.Info.TaskType, reply.Info.Id)
		var accept bool
		switch reply.Info.TaskType {
		case MAP:
			{
				accept = doMapTask(reply.Info.Id, reply.Info.Filename, reply.NReduce, mapf)
			}
		case REDUCE:
			{
				accept = doReduceTask(reply.Info.Id, reply.NMap, reducef)
			}
		case QUIT:
			{
				log.Println("worker received QUIT job from Coordinator")
				goto END
			}
		}
		if accept {
			log.Printf("%s task %d accepted by Coordinator\n", reply.Info.TaskType, reply.Info.Id)
		} else {
			log.Printf("%s task %d rejected by Coordinator\n", reply.Info.TaskType, reply.Info.Id)
		}
	}
END:
	log.Printf("worker %d quit working\n", id)
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

// do map task :
// 1. generate intermediate output
// 2. output intermediate to file (use temp file)
// 3. report to coordinator
func doMapTask(taskId int, filename string, nReduce int, mapf func(string, string) []KeyValue) bool {
	// do map task && generate intermediate file
	intermediate := generateIntermediate(filename, mapf)
	// write intermediate to file
	writeToFile(taskId, nReduce, intermediate)
	// report to coordinator
	accept := reportTask(MAP, taskId)
	return accept
}

func generateIntermediate(filename string, mapf func(string, string) []KeyValue) []KeyValue {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	intermediate := mapf(filename, string(content))
	return intermediate
}

func writeToFile(taskId int, nReduce int, intermediate []KeyValue) {
	hashedkva := make(map[int][]KeyValue)
	for _, kv := range intermediate {
		hashedIndex := ihash(kv.Key) % nReduce
		hashedkva[hashedIndex] = append(hashedkva[hashedIndex], kv)
	}

	for i := 0; i < nReduce; i++ {
		tmpFile, err := os.CreateTemp(".", "mrtmp"+time.Now().String())
		if err != nil {
			log.Fatal(err)
		}
		for _, kv := range hashedkva[i] {
			fmt.Fprintf(tmpFile, "%v\t%v\n", kv.Key, kv.Value)
		}
		outname := tmpMapOutFile(taskId, i)
		err = os.Rename(tmpFile.Name(), outname)
		if err != nil {
			log.Fatalf("cannot rename temp file to %v", outname)
		}
		tmpFile.Close()
	}
}

func reportTask(taskType string, taskID int) bool {
	args, reply := ReportTaskArgs{
		TaskID:   taskID,
		TaskType: taskType,
	}, ReportTaskReply{}
	call("Coordinator.ReportTask", &args, &reply)

	return reply.Accept
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// do reduce task
func doReduceTask(taskId int, nMap int, reducef func(string, []string) string) bool {
	lines := readIntermediate(taskId, nMap)

	var kva []KeyValue
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sp := strings.Split(line, "\t")
		kva = append(kva, KeyValue{
			Key:   sp[0],
			Value: sp[1],
		})
	}

	sort.Sort(ByKey(kva))

	filename := tmpReduceOutFile(taskId)
	outFile, _ := os.CreateTemp(".", "mrtmp-reduce-"+filename)

	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ {
			values = append(values, kva[k].Value)
		}
		output := reducef(kva[i].Key, values)

		fmt.Fprintf(outFile, "%v %v\n", kva[i].Key, output)
		i = j
	}
	outFile.Close()
	err := os.Rename(outFile.Name(), filename)
	if err != nil {
		log.Fatalf("cannot rename temp file to %v", filename)
	}
	accept := reportTask(REDUCE, taskId)
	return accept
}

func readIntermediate(taskId int, nMap int) []string {
	var lines []string
	for i := 0; i < nMap; i++ {
		filename := finalMapOutFile(i, taskId)
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("file open failed : %s", filename)
		}
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("file read failed : %s", filename)
		}
		file.Close()
		lines = append(lines, strings.Split(string(content), "\n")...)
	}
	return lines
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
