package chaperone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
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

func BenchmarkGraph(b *testing.B) {
	ctx := context.Background()

	SupervisorName := "supervisor1"
	Node1Name := "node1"
	Node2Name := "node2"
	Node3Name := "node3"
	Node4Name := "node4"
	Node5Name := "node5"
	BufferSize := 100_000_000
	inputChannelName := "input"
	outputChannelName := "output"
	numberWorkers := 4
	totalMessages := 5_965_232

	graph := NewGraph[benchmarkMessage](ctx).
		AddSupervisor(SupervisorName, &benchmarkSupervisorHandler{}).
		AddNode(SupervisorName, Node1Name, &benchmarkHandler{outChannelName: Node1Name + ":" + outputChannelName}).
		AddNode(SupervisorName, Node2Name, &benchmarkHandler{outChannelName: Node2Name + ":" + outputChannelName}).
		AddNode(SupervisorName, Node3Name, &benchmarkHandler{outChannelName: Node3Name + ":" + outputChannelName}).
		AddNode(SupervisorName, Node4Name, &benchmarkHandler{outChannelName: Node4Name + ":" + outputChannelName}).
		AddNode(SupervisorName, Node5Name, &benchmarkHandler{outChannelName: Node5Name + ":" + outputChannelName}).
		AddEdge("", "", Node1Name, inputChannelName, BufferSize, numberWorkers).
		AddEdge(Node1Name, outputChannelName, Node2Name, inputChannelName, BufferSize, numberWorkers).
		AddEdge(Node2Name, outputChannelName, Node3Name, inputChannelName, BufferSize, numberWorkers).
		AddEdge(Node3Name, outputChannelName, Node4Name, inputChannelName, BufferSize, numberWorkers).
		AddEdge(Node4Name, outputChannelName, Node5Name, inputChannelName, BufferSize, numberWorkers).
		AddEdge(Node5Name, outputChannelName, "", "final", BufferSize, numberWorkers).
		Start()

	fmt.Println("Graph started")
	fmt.Printf("Number of nodes: %d\n", len(graph.Nodes))

	for i := 0; i < 10; i++ {
		// Run the benchmark
		b.ResetTimer()
		sendCount := 0
		startTime := time.Now()

		wg := sync.WaitGroup{}
		wg.Add(totalMessages)

		go func() {
			for i := 0; i < totalMessages; i++ {
				msg := &benchmarkMessage{}
				msg.SetContent("test message")
				env := &Envelope[benchmarkMessage]{message: msg}
				graph.Nodes[Node1Name].inputChans[Node1Name+":"+inputChannelName] <- env
				sendCount++
			}
		}()

		doneCount := 0
		for range graph.Nodes[Node5Name].outputChans[Node5Name+":"+outputChannelName].goChans[":final"] {
			doneCount++
			wg.Done()
			if doneCount == totalMessages {
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
		b.StopTimer()

		envelopesPerSecond := float64(doneCount) / elapsedTime
		fmt.Printf("Processed %f envelopes per second for a bitrate of %d Mbps\n", envelopesPerSecond, bitrate(envelopesPerSecond))
	}
	graph.Stop()
}

func bitrate(eps float64) int {
	return int(eps * 180 * 8 / 1024 / 1024)
}
