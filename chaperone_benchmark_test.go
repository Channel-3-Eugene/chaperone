package chaperone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"
)

type benchmarkMessage struct {
	Content [180]byte
}

func (msg *benchmarkMessage) SetContent(content string) {
	copy(msg.Content[:], content)
}

func (msg *benchmarkMessage) GetContent() string {
	return string(bytes.Trim(msg.Content[:], "\x00"))
}

type benchmarkHandler struct {
	outChannelName string
}

func (h *benchmarkHandler) Handle(msg *benchmarkMessage) (string, error) {
	if msg.GetContent() == "error" {
		return "", errors.New("test error")
	}
	return h.outChannelName, nil
}

type benchmarkSupervisorHandler struct{}

func (h *benchmarkSupervisorHandler) Handle(msg *benchmarkMessage) (string, error) {
	fmt.Printf("Supervisor received message: %s\n", msg.Content)
	return "supervised", nil
}

func logMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func startMemoryProfiling() func() {
	memFile, err := os.Create("mem.prof")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	return func() {
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		memFile.Close()
	}
}

func startCPUProfiling() func() {
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	return func() {
		pprof.StopCPUProfile()
		cpuFile.Close()
	}
}

func BenchmarkGraph(b *testing.B) {
	stopCPUProfiling := startCPUProfiling()
	defer stopCPUProfiling()

	stopMemoryProfiling := startMemoryProfiling()
	defer stopMemoryProfiling()

	ctx := context.Background()

	SupervisorName := "supervisor1"
	Node1Name := "node1"
	Node1WorkerName := "worker1"
	Node2Name := "node2"
	Node2WorkerName := "worker2"
	Node3Name := "node3"
	Node3WorkerName := "worker3"
	Node4Name := "node4"
	Node4WorkerName := "worker4"
	Node5Name := "node5"
	Node5WorkerName := "worker5"
	BufferSize := 1_000_000
	inputChannelName := "input"
	outputChannelName := "outChannel"
	numberWorkers := 8

	graph := NewGraph[benchmarkMessage](ctx).
		AddSupervisor(SupervisorName, &benchmarkSupervisorHandler{}).
		AddNode(SupervisorName, Node1Name, &benchmarkHandler{outChannelName: Node1Name + ":" + outputChannelName}).
		AddWorkers(Node1Name, numberWorkers, Node1WorkerName).
		AddNode(SupervisorName, Node2Name, &benchmarkHandler{outChannelName: Node2Name + ":" + outputChannelName}).
		AddWorkers(Node2Name, numberWorkers, Node2WorkerName).
		AddNode(SupervisorName, Node3Name, &benchmarkHandler{outChannelName: Node3Name + ":" + outputChannelName}).
		AddWorkers(Node3Name, numberWorkers, Node3WorkerName).
		AddNode(SupervisorName, Node4Name, &benchmarkHandler{outChannelName: Node4Name + ":" + outputChannelName}).
		AddWorkers(Node4Name, numberWorkers, Node4WorkerName).
		AddNode(SupervisorName, Node5Name, &benchmarkHandler{outChannelName: Node5Name + ":" + outputChannelName}).
		AddWorkers(Node5Name, numberWorkers, Node5WorkerName).
		AddEdge("", "", Node1Name, inputChannelName, BufferSize).
		AddEdge(Node1Name, outputChannelName, Node2Name, inputChannelName, BufferSize).
		AddEdge(Node2Name, outputChannelName, Node3Name, inputChannelName, BufferSize).
		AddEdge(Node3Name, outputChannelName, Node4Name, inputChannelName, BufferSize).
		AddEdge(Node4Name, outputChannelName, Node5Name, inputChannelName, BufferSize).
		AddEdge(Node5Name, outputChannelName, "", "final", BufferSize).
		Start()

	fmt.Println("Graph started")
	fmt.Printf("Number of nodes: %d\n", len(graph.Nodes))

	// Run the benchmark
	b.ResetTimer()
	sendCount := 0
	startTime := time.Now()

	wg := sync.WaitGroup{}
	wg.Add(b.N)

	go func() {
		for i := 0; i < b.N; i++ {
			msg := &benchmarkMessage{}
			msg.SetContent("test message")
			env := &Envelope[benchmarkMessage]{message: msg}
			graph.Nodes[Node1Name].inputChans[inputChannelName] <- env
			sendCount++
		}
	}()

	doneCount := 0
	for range graph.Nodes[Node5Name].outputChans[Node5Name+":"+outputChannelName].goChans[":final"] {
		doneCount++
		wg.Done()
		if doneCount == b.N {
			break
		}
		for _, edge := range graph.Edges {
			if len(edge.Channel) == cap(edge.Channel) {
				fmt.Printf("Buffer for channel %s full: %d\n", edge.Destination, len(edge.Channel))
			}
		}
	}

	logMemoryUsage()

	wg.Wait()
	elapsedTime := time.Since(startTime).Seconds()
	fmt.Printf("\n%d messages processed\n", doneCount)
	b.StopTimer()

	graph.Stop()

	envelopesPerSecond := float64(doneCount) / elapsedTime
	fmt.Printf("Processed %f envelopes per second for a bitrate of %d Mbps\n", envelopesPerSecond, bitrate(envelopesPerSecond))
}

func bitrate(eps float64) int {
	return int(eps * 180 * 8 / 1024 / 1024)
}
