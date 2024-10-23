package testing

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "os"
    "sync"
    "time"
    h "web3server/helper"
    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/ethclient"
)

// NetworkCondition simulates different network scenarios
type NetworkCondition struct {
    BaseLatency time.Duration
    Jitter     time.Duration
    PacketLoss float64
}

// TestConfig holds the configuration for our test setup
type TestConfig struct {
    NetworkEndpoint    string
    ContractAddress    common.Address
    ContractABI        abi.ABI
    NetworkConditions  []NetworkCondition
}

// TestNode represents a single listener with its network conditions
type TestNode struct {
    ID               int
    Client           *ethclient.Client
    ContractABI      abi.ABI
    EventTimes       map[string]time.Time
    EventData        map[string]interface{}
    NetworkCondition NetworkCondition
    mu              sync.RWMutex
    logger          *log.Logger
}

// DistributedTester manages multiple nodes for testing
type DistributedTester struct {
    Nodes          []*TestNode
    Config         TestConfig
    StartTime      time.Time
    wg             sync.WaitGroup
}

// NewDistributedTester creates a new test setup with multiple listeners
func NewDistributedTester(config TestConfig) (*DistributedTester, error) {
    var nodes []*TestNode
    
    for i, condition := range config.NetworkConditions {
        client, err := ethclient.Dial(config.NetworkEndpoint)
        if err != nil {
            return nil, fmt.Errorf("failed to connect node %d: %w", i, err)
        }
        
        nodes = append(nodes, &TestNode{
            ID:               i,
            Client:           client,
            ContractABI:      config.ContractABI,
            EventTimes:       make(map[string]time.Time),
            EventData:        make(map[string]interface{}),
            NetworkCondition: condition,
            logger:          log.New(os.Stdout, fmt.Sprintf("[Node %d] ", i), log.LstdFlags|log.Lmicroseconds),
        })
    }
    
    return &DistributedTester{
        Nodes:  nodes,
        Config: config,
    }, nil
}

// simulateNetworkConditions adds artificial delay and packet loss
func (n *TestNode) simulateNetworkConditions() {
    // Simulate network latency
    baseDelay := n.NetworkCondition.BaseLatency
    jitter := time.Duration(rand.Int63n(int64(n.NetworkCondition.Jitter)))
    totalDelay := baseDelay + jitter

    // Simulate packet loss
    if rand.Float64() < n.NetworkCondition.PacketLoss {
        // Simulate packet loss by adding substantial delay
        totalDelay += 5 * time.Second
        n.logger.Printf("Simulated packet loss, adding 5s delay")
    }

    time.Sleep(totalDelay)
}

// StartEventMonitoring starts monitoring events across all nodes
func (dt *DistributedTester) StartEventMonitoring(ctx context.Context) error {
    dt.StartTime = time.Now()
    
    query := ethereum.FilterQuery{
        Addresses: []common.Address{dt.Config.ContractAddress},
    }
    
    for _, node := range dt.Nodes {
        dt.wg.Add(1)
        go func(n *TestNode) {
            defer dt.wg.Done()
            
            logs := make(chan types.Log)
            sub, err := n.Client.SubscribeFilterLogs(ctx, query, logs)
            if err != nil {
                n.logger.Printf("Failed to subscribe to events: %v", err)
                return
            }
            defer sub.Unsubscribe()
            
            n.logger.Printf("Started monitoring with conditions: Latency=%v, Jitter=%v, PacketLoss=%.2f%%",
                n.NetworkCondition.BaseLatency,
                n.NetworkCondition.Jitter,
                n.NetworkCondition.PacketLoss*100)
            
            for {
                select {
                case vLog := <-logs:
                    n.handleLog(vLog)
                    
                case err := <-sub.Err():
                    n.logger.Printf("Subscription error: %v", err)
                    // Attempt to resubscribe
                    time.Sleep(5 * time.Second)
                    sub, err = n.Client.SubscribeFilterLogs(ctx, query, logs)
                    if err != nil {
                        n.logger.Printf("Failed to resubscribe: %v", err)
                        return
                    }
                    
                case <-ctx.Done():
                    return
                }
            }
        }(node)
    }
    
    return nil
}

// handleLog processes incoming events for a node
func (n *TestNode) handleLog(vLog types.Log) {
    // Simulate network conditions before processing
    n.simulateNetworkConditions()

    receiveTime := time.Now()
    n.mu.Lock()
    defer n.mu.Unlock()
    
    txHash := vLog.TxHash.Hex()
    n.EventTimes[txHash] = receiveTime
    
    switch vLog.Topics[0].Hex() {
    case n.ContractABI.Events["PushEncryptedData"].ID.Hex():
        var event h.PushEncrytedDataEvent
        err := n.ContractABI.UnpackIntoInterface(&event, "PushEncryptedData", vLog.Data)
        if err != nil {
            n.logger.Printf("Failed to unpack PushEncryptedData event: %v", err)
            return
        }
        n.EventData[txHash] = event
        n.logger.Printf("Received PushEncryptedData event\nOwner: %s\nDataName: %s",
            event.Owner,
            event.DataName)
            
    case n.ContractABI.Events["PushPrivateKey"].ID.Hex():
        var event h.PushPrivateKeyEvent
        err := n.ContractABI.UnpackIntoInterface(&event, "PushPrivateKey", vLog.Data)
        if err != nil {
            n.logger.Printf("Failed to unpack PushPrivateKey event: %v", err)
            return
        }
        n.EventData[txHash] = event
        n.logger.Printf("Received PushPrivateKey event\nOwner: %s\nDataName: %s",
            event.Owner,
            event.DataName)
    }
}

// GetEventStats returns timing statistics for events across nodes
func (dt *DistributedTester) GetEventStats() map[string]struct {
    FirstNode     int
    LastNode      int
    TimeDiff      time.Duration
    NodeTimings   map[int]time.Time
    EventType     string
    EventData     interface{}
} {
    stats := make(map[string]struct {
        FirstNode     int
        LastNode      int
        TimeDiff      time.Duration
        NodeTimings   map[int]time.Time
        EventType     string
        EventData     interface{}
    })
    
    for _, node := range dt.Nodes {
        node.mu.RLock()
        for txHash, receiveTime := range node.EventTimes {
            if _, exists := stats[txHash]; !exists {
                stats[txHash] = struct {
                    FirstNode     int
                    LastNode      int
                    TimeDiff      time.Duration
                    NodeTimings   map[int]time.Time
                    EventType     string
                    EventData     interface{}
                }{
                    NodeTimings: make(map[int]time.Time),
                    EventData:   node.EventData[txHash],
                }
            }
            
            entry := stats[txHash]
            entry.NodeTimings[node.ID] = receiveTime
            stats[txHash] = entry
        }
        node.mu.RUnlock()
    }
    
    // Calculate time differences and identify first/last nodes
    for txHash, stat := range stats {
        var firstTime, lastTime time.Time
        firstNode, lastNode := -1, -1
        
        for nodeID, timing := range stat.NodeTimings {
            if firstTime.IsZero() || timing.Before(firstTime) {
                firstTime = timing
                firstNode = nodeID
            }
            if lastTime.IsZero() || timing.After(lastTime) {
                lastTime = timing
                lastNode = nodeID
            }
        }
        
        stat.FirstNode = firstNode
        stat.LastNode = lastNode
        stat.TimeDiff = lastTime.Sub(firstTime)
        stats[txHash] = stat
    }
    
    return stats
}

// Close closes all client connections
func (dt *DistributedTester) Close() {
    for _, node := range dt.Nodes {
        node.Client.Close()
    }
}
