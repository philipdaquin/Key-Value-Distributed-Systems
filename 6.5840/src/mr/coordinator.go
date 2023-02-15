package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)


type MapTasks struct { 
	id int
	file string 
	startAt time.Time
	isDone bool
}

type ReduceTasks struct { 
	id int 
	files []string
	startAt time.Time
	isDone bool 
}


type Coordinator struct {
	locks sync.Mutex
	reduceTasks []ReduceTasks
	mapTasks []MapTasks
	mapLeft int 
	reduceLeft int

}

/*
	The Coordinator is responsible for coordinating the Map and Reduce Operations 
	in a MapReduce setting 

	1. Divide the input data into smaller chunks and assign each chunk to a worker
		for processing
	
	2. Schedule Map task. The coordinator schedules Map tasks to run on each workers node

	3. Collect intermediate result
		After the Map tasks have completed, the coordinator collects the intermediate results 
		generated bu each worker node 

		These intermediate results are stored as key-value pairs, where the key is the intermediate 
		result key and the value is the intermediate result value
	
	4. Group intermediate result by key. 
		The coordinator groups the intermediate results by key 
		and assigns groups of intermediate results to Reduce Task.
		Each reduce task will process a group of intermediate result with the same key

	5. Schedule reduce task -> The coordinator schedules the reduce tasks to run on worker nodes
		Each reduce task applies the reduce function to the intermediate results assigned to the task 
	
	6. Collect and aggregate final results 
		After the reduce tasks have completed , the completed collects and aggregates the final results generated by each 
		reduce task 
*/
func (self *Coordinator) GetTask(args *TaskArgs, reply *TaskReply) error {

	fmt.Println("👁️👁️ Coordinator")

	self.locks.Lock()

	defer self.locks.Unlock()

	//	Task: Map 
	//
	// 	Collect intermediate results
	// 	After the Map tasks have completed, the coordinator collects 
	// 	the intermediate results generated by each worker node. 
	if args.WorkerStatus == Map {
		// 
		// 	The coordinator groups the intermediate results by key 
		// 	and assigns groups of intermediate results to Reduce tasks.
		//
		if !self.mapTasks[args.WorkerId].isDone {
			self.mapTasks[args.WorkerId].isDone = true
			//  Map Task -> Reduce Tasks
			for id, completedTasks := range args.CompletedTasks { 
				if len(completedTasks) > 0  { 
					self.reduceTasks[id].files = append(self.reduceTasks[id].files, completedTasks)
				}
			}
			self.mapLeft -=1;
		} 
	}
	//	Task: Reduce 
	//	The coordinator updates its records of the Reduce tasks
	//	by marking it as dne and decrements the number of remaining Reduce Tasks 
	if args.WorkerStatus == Reduce {
		if !self.reduceTasks[args.WorkerId].isDone {
			self.reduceTasks[args.WorkerId].isDone = true
			self.reduceLeft -=1
		}
	}

	// 	Determine what the next task for the worker should be
	// 
	//  Check for remaining Map tasks tasks, and check if the worker has been running 
	//	running longer than 10 seconds. 
	//		- If any of them have, it assigns the task to the worker and fills in 
	//		  `reply` with the necessary information 
	now := time.Now()
	timeLeft := now.Add(time.Second * -10)

	// Check for remaining Map tasks left
	if self.mapLeft > 0 {
		// Check if the worker has been running longer than 10 seconds
		for MapWorkerId, _ := range self.mapTasks {
			mapWorker := self.mapTasks[MapWorkerId]
			// Skip finished workers 
			if mapWorker.isDone { continue }

			// Check if the startTime has been running longer than 10s
			if mapWorker.startAt.Before(timeLeft) {
				newTask := TaskReply{
					WorkerId: mapWorker.id,
					WorkerStatus: Map,
					ImpendingTasks: []string{mapWorker.file},
					NReduce: len(self.reduceTasks),
				}
				mapWorker.startAt = now
				
				reply = &newTask
				return nil
			}
		}
		// If mapTasks is empty
		reply.WorkerStatus = Sleep
	} 
	
	if self.reduceLeft > 0 {
		for ReduceWorkerId, _ := range self.reduceTasks {
			reduceWorker := self.reduceTasks[ReduceWorkerId]
			// Skip done workers 
			if reduceWorker.isDone { continue }

			if reduceWorker.startAt.Before(timeLeft) {
				reply.ImpendingTasks = reduceWorker.files
				reply.WorkerId = reduceWorker.id
				reply.WorkerStatus = Reduce

				reduceWorker.startAt = now

				return nil
			}
		}
		// If reduceTask is empty
		reply.WorkerStatus = Sleep
	} 
	
	// Terminate itself
	reply.WorkerStatus = Exit
	
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {

	fmt.Println("✅ Welcome to this server!!!!!")

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

	fmt.Println("✅ Coordinator Done!")

	c.locks.Lock()
	defer c.locks.Unlock()
	return c.mapLeft == 0 && c.reduceLeft == 0
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	
	if len(files) == 0 {
		log.Fatalf("❌ Empty Files")
	}
	
	
	c := Coordinator{

		mapTasks: make([]MapTasks, len(files)),
		reduceTasks: make([]ReduceTasks, nReduce),
		mapLeft: len(files),
		reduceLeft: nReduce,
	}

	// Your code here.
	
	
	// Intialise Map 
	for idx, file := range files { 
		c.mapTasks[idx] = MapTasks{id: idx, file: file, isDone: false}
	}


	// Initialise Reduce 
	for idx := 0; idx < nReduce; idx +=1 { 
		c.reduceTasks[idx] = ReduceTasks{id: idx, isDone: false}
	}




	c.server()
	return &c
}
