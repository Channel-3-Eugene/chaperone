package chaperone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

const NUMRUNS = 10

type benchmarkMessage struct {
	Content [180]byte
}

func (m benchmarkMessage) String() string {
	return string(bytes.Trim(m.Content[:], "\x00"))
}

func (msg *benchmarkMessage) SetContent(content string) {
	copy(msg.Content[:], content)
}

type benchmarkHandler struct {
	outChannelName string
}

func (h *benchmarkHandler) Start(context.Context) error {
	return nil
}

func (h *benchmarkHandler) Handle(_ context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}

	return env, nil
}

type benchmarkSupervisorHandler struct{}

func (h *benchmarkSupervisorHandler) Start(context.Context) error {
	return nil
}

func (h *benchmarkSupervisorHandler) Handle(_ context.Context, _ Message) error {
	return nil
}

func BenchmarkGraph(b *testing.B) {
	ctx := context.Background()

	bufferSize := 500_000_000
	numWorkers := 2
	totalMessages := 10_000_000

	SupervisorName := "supervisor1"
	Supervisor := NewSupervisor(ctx, SupervisorName, &benchmarkSupervisorHandler{})
	Node1Name := "node1"
	Node1 := NewNode[benchmarkMessage, benchmarkMessage](ctx, Node1Name, &benchmarkHandler{outChannelName: "out1"})
	Edge0 := NewEdge("in", nil, Node1, bufferSize, numWorkers)
	Node2Name := "node2"
	Node2 := NewNode[benchmarkMessage, benchmarkMessage](ctx, Node2Name, &benchmarkHandler{outChannelName: "out2"})
	Edge1 := NewEdge("out1", Node1, Node2, bufferSize, numWorkers)
	Node3Name := "node3"
	Node3 := NewNode[benchmarkMessage, benchmarkMessage](ctx, Node3Name, &benchmarkHandler{outChannelName: "out3"})
	Edge2 := NewEdge("out2", Node2, Node3, bufferSize, numWorkers)
	Node4Name := "node4"
	Node4 := NewNode[benchmarkMessage, benchmarkMessage](ctx, Node4Name, &benchmarkHandler{outChannelName: "out4"})
	Edge3 := NewEdge("out3", Node3, Node4, bufferSize, numWorkers)
	Node5Name := "node5"
	Node5 := NewNode[benchmarkMessage, benchmarkMessage](ctx, Node5Name, &benchmarkHandler{outChannelName: "out"})
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

	fmt.Println("Graph started")
	fmt.Printf("Number of nodes: %d\n", len(graph.Nodes))

	startTime := time.Now()
	totalEnvelopes := 0

	for i := 0; i < NUMRUNS; i++ {
		sendCount := 0
		startTime := time.Now()

		wg := sync.WaitGroup{}
		wg.Add(totalMessages)

		go func() {
			for i := 0; i < totalMessages; i++ {
				msg := benchmarkMessage{}
				msg.SetContent("test message")
				env := &Envelope[benchmarkMessage]{Message: msg}
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

		wg.Wait()
		elapsedTime := time.Since(startTime).Seconds()

		envelopesPerSecond := float64(doneCount) / elapsedTime
		totalEnvelopes += doneCount

		fmt.Printf("Processed %d packets at %f packets per second for a bitrate of %d Mbps\n", totalMessages, envelopesPerSecond, bitrate(envelopesPerSecond))
	}

	elapsedTime := time.Since(startTime).Seconds()
	graph.Stop()

	sumEnvelopesPerSecond := float64(totalEnvelopes) / elapsedTime
	fmt.Printf("Average %f envelopes per second for a bitrate of %d Mbps over %d runs\n", sumEnvelopesPerSecond, bitrate(sumEnvelopesPerSecond), NUMRUNS)
}

func bitrate(eps float64) int {
	return int(eps * 180 * 8 / 1024 / 1024)
}
