# Lab1 MapReduce

简单记录一下设计：

- 系统中的 worker 设计为无状态的，coordinator 只关心任务的状态，并且当前系统的任务状态由 coordinator来维护。这也意味着 worker 的地位是对等的，它只负责向 coordinator 申请任务并且执行，根据任务的 taskType 进行判断自己应该做什么

- 根据这种设计，map 和 reduce 的结果文件名需要设计才能保证作业完成。中间结果的合并由 coordinator 来完成，关键字涉及到 mapId 、 reduceId，再加上他们的 taskType 来表示一个对应的结果。

- worker 在完成任务后，需要向 coordinator report 以更新任务的状态

- coordinator 要定期检查超时的任务，这个列表由它自己维护。这个已分发的任务列表会在收到 report 的时候更新