//go:build profile

package chaperone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"
)

type profileMessage struct {
	Content [180]byte
}

func (m profileMessage) String() string {
	return string(bytes.Trim(m.Content[:], "\x00"))
}

func (msg *profileMessage) SetContent(content string) {
	copy(msg.Content[:], content)
}

type profileHandler struct {
	outChannelName string
}

func (h *profileHandler) Start(context.Context) error {
	return nil
}

func (h *profileHandler) Handle(_ context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}

	return env, nil
}

type profileSupervisorHandler struct{}

func (h *profileSupervisorHandler) Start(context.Context) error {
	return nil
}

func (h *profileSupervisorHandler) Handle(_ context.Context, evt Message) error {
	return nil
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

func TestProfilingGraph(t *testing.T) {
	// Start CPU profiling
	cpuProfile, err := os.Create("cpu_profile.prof")
	if err != nil {
		t.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()
	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		t.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	ctx := context.Background()

	numWorkers := 4
	bufferSize := 100_000_000
	totalMessages := 5_965_232

	SupervisorName := "supervisor1"
	Supervisor := NewSupervisor(ctx, SupervisorName, &profileSupervisorHandler{})
	Node1Name := "node1"
	Node1 := NewNode[profileMessage, profileMessage](ctx, Node1Name, &profileHandler{outChannelName: "out1"})
	Edge0 := NewEdge("in", nil, Node1, bufferSize, numWorkers)
	Node2Name := "node2"
	Node2 := NewNode[profileMessage, profileMessage](ctx, Node2Name, &profileHandler{outChannelName: "out2"})
	Edge1 := NewEdge("out1", Node1, Node2, bufferSize, numWorkers)
	Node3Name := "node3"
	Node3 := NewNode[profileMessage, profileMessage](ctx, Node3Name, &profileHandler{outChannelName: "out3"})
	Edge2 := NewEdge("out2", Node2, Node3, bufferSize, numWorkers)
	Node4Name := "node4"
	Node4 := NewNode[profileMessage, profileMessage](ctx, Node4Name, &profileHandler{outChannelName: "out4"})
	Edge3 := NewEdge("out3", Node3, Node4, bufferSize, numWorkers)
	Node5Name := "node5"
	Node5 := NewNode[profileMessage, profileMessage](ctx, Node5Name, &profileHandler{outChannelName: "out"})
	Edge4 := NewEdge("out4", Node4, Node5, bufferSize, numWorkers)
	Edge5 := NewEdge("out", Node5, nil, bufferSize, numWorkers)

	graph := NewGraph(ctx, "graph", &Config{}).
		AddSupervisor(nil, Supervisor).
		AddEdge(Edge0).
		AddNode(Supervisor, Node1).
		AddEdge(Edge1).
		AddNode(Supervisor, Node2).
		AddEdge(Edge2).
		AddNode(Supervisor, Node3).
		AddEdge(Edge3).
		AddNode(Supervisor, Node4).
		AddEdge(Edge4).
		AddNode(Supervisor, Node5).
		AddEdge(Edge5).
		Start()

	sendCount := 0
	startTime := time.Now()

	wg := sync.WaitGroup{}
	wg.Add(totalMessages)

	go func() {
		for i := 0; i < totalMessages; i++ {
			msg := profileMessage{}
			msg.SetContent("test message")
			env := &Envelope[profileMessage]{Message: msg}
			Edge0.GetChannel() <- env
			sendCount++
		}
	}()

	doneCount := 0
	for range Edge5.GetChannel() {
		doneCount++
		wg.Done()
		if doneCount == totalMessages {
			break
		}
		for _, edge := range graph.Edges {
			if len(edge.GetChannel()) == cap(edge.GetChannel()) {
				fmt.Printf("Buffer for channel %s full: %d\n", edge.Name(), len(edge.GetChannel()))
			}
		}
	}

	logMemoryUsage()

	wg.Wait()
	elapsedTime := time.Since(startTime).Seconds()

	envelopesPerSecond := float64(doneCount) / elapsedTime
	fmt.Printf("Processed %f envelopes per second for a bitrate of %d Mbps\n", envelopesPerSecond, bitrate(envelopesPerSecond))

	graph.Stop()

	// Start memory profiling
	memProfile, err := os.Create("mem_profile.prof")
	if err != nil {
		t.Fatalf("could not create memory profile: %v", err)
	}
	defer memProfile.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(memProfile); err != nil {
		t.Fatalf("could not write memory profile: %v", err)
	}
}
