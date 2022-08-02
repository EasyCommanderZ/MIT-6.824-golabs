package mr

import "fmt"

func tmpMapOutFile(taskID int, mapId int, reduceId int) string {
	return fmt.Sprintf("intermediate-map-%d-%d-%d", taskID, mapId, reduceId)
}

func finalMapOutFile(mapId int, reduceId int) string {
	return fmt.Sprintf("mr-map-%d-%d", mapId, reduceId)
}

func tmpReduceOutFile(reduceId int) string {
	return fmt.Sprintf("intermeidate-reduce-%d", reduceId)
}

func finalReduceOutFile(reduceId int) string {
	return fmt.Sprintf("mr-out-%d", reduceId)
}
