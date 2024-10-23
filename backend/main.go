package main

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "math/big"
    "strconv"
    "strings"
    "time"
    h "web3server/helper"
    t "web3server/testing"
    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/common"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
)

type Contract struct {
    ABI json.RawMessage `json:"abi"`
}

// Event structs to match contract events
type ReleaseEncryptedDataEvent struct {
    EncryptedData []byte
    Owner         string
    DataName      string
    ReleaseTime   *big.Int
    Hash          []byte
}

type KeyReleasedEvent struct {
    PrivateKey []byte
    Owner      string
    DataName   string
}

type KeyReleaseRequestedEvent struct {
    Index    *big.Int
    Owner    string
    DataName string
}

var (
    client          *ethclient.Client
    contractAddress common.Address
    contractABI     abi.ABI
    PrivateKey      string
    distributor     *t.DistributedTester
)

func LoadABI() (abi.ABI, error) {
    filePath := "TwoPhaseCommit.json"
    abiBytes, err := os.ReadFile(filePath)
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to read ABI file: %v", err)
    }

    var contract Contract
    err = json.Unmarshal(abiBytes, &contract)
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to unmarshal contract JSON: %v", err)
    }

    parsedABI, err := abi.JSON(strings.NewReader(string(contract.ABI)))
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to parse ABI JSON: %v", err)
    }

    return parsedABI, nil
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatalf("Error loading .env file")
    }
    PrivateKey = "0x" + MustGetEnv("PRIVATE_KEY")

    var err error
    client, err = ethclient.Dial("wss://api.avax-test.network/ext/bc/C/ws")
    if err != nil {
        log.Fatalf("Failed to connect to the Ethereum client: %v", err)
    }
    defer client.Close()

    contractAddress = common.HexToAddress("0xEA0243082093B09858b37f08d30531a29cA6589b")
    contractABI, err = LoadABI()
    if err != nil {
        log.Fatalf("Failed to parse contract ABI: %v", err)
    }

    // Set up distributed testing configuration
    testConfig := t.TestConfig{
        NetworkEndpoint: "wss://api.avax-test.network/ext/bc/C/ws",
        ContractAddress: contractAddress,
        ContractABI:     contractABI,
        NetworkConditions: []t.NetworkCondition{
            {
                BaseLatency: 100 * time.Millisecond,
                Jitter:     50 * time.Millisecond,
                PacketLoss: 0.01, // 1% packet loss
            },
            {
                BaseLatency: 200 * time.Millisecond,
                Jitter:     100 * time.Millisecond,
                PacketLoss: 0.05, // 5% packet loss
            },
            {
                BaseLatency: 500 * time.Millisecond,
                Jitter:     200 * time.Millisecond,
                PacketLoss: 0.1, // 10% packet loss
            },
        },
    }

    // Initialize distributed tester
    distributor, err = t.NewDistributedTester(testConfig)
    if err != nil {
        log.Fatalf("Failed to initialize distributed tester: %v", err)
    }
    defer distributor.Close()

    // Start monitoring events across all test nodes
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := distributor.StartEventMonitoring(ctx); err != nil {
        log.Fatalf("Failed to start event monitoring: %v", err)
    }

    // Start the regular Web3 listener
    go Web3Listener()

    router := gin.Default()
    router.POST("/upload", postData)
    router.GET("/get/:dataname/:owner", getData)
    router.GET("/stats", getTestingStats)

    fmt.Println("Server is running on port 8080")
    router.Run(":8080")
}

func getTestingStats(c *gin.Context) {
    stats := distributor.GetEventStats()
    
    formattedStats := make(map[string]map[string]interface{})
    
    for txHash, stat := range stats {
        formattedStats[txHash] = map[string]interface{}{
            "first_node":        stat.FirstNode,
            "last_node":         stat.LastNode,
            "time_difference_ms": stat.TimeDiff.Milliseconds(),
            "node_timings":      formatTimings(stat.NodeTimings),
            "event_data":        stat.EventData,
        }
    }
    
    c.JSON(200, formattedStats)
}

func formatTimings(timings map[int]time.Time) map[string]string {
    formatted := make(map[string]string)
    for nodeID, timing := range timings {
        formatted[fmt.Sprintf("node_%d", nodeID)] = timing.Format(time.RFC3339Nano)
    }
    return formatted
}

func postData(c *gin.Context) {
    data := c.PostForm("data")
    owner := c.PostForm("owner")
    dataName := c.PostForm("dataname")
    releaseTime := c.PostForm("releaseTime")

    // Encrypt the data first
    encryptedData, _, err := h.EncryptData(data)
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to encrypt data: %v", err)})
        return
    }

    // Calculate hash from the encrypted data
    hash := sha256.Sum256(encryptedData)

    ReleaseTime, err := strconv.ParseUint(releaseTime, 10, 64)
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to convert release time to uint64: %v", err)})
        return
    }

    // Validate input parameters
    if len(encryptedData) == 0 {
        c.JSON(400, gin.H{"error": "Encrypted data cannot be empty"})
        return
    }
    if len(owner) == 0 {
        c.JSON(400, gin.H{"error": "Owner cannot be empty"})
        return
    }
    if len(dataName) == 0 {
        c.JSON(400, gin.H{"error": "Data name cannot be empty"})
        return
    }
    if ReleaseTime <= uint64(time.Now().Unix()) {
        c.JSON(400, gin.H{"error": "Release time must be in the future"})
        return
    }

    privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(PrivateKey, "0x"))
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to parse private key: %v", err)})
        return
    }

    auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(43113))
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to create authenticated transactor: %v", err)})
        return
    }

    input, err := contractABI.Pack("addStoredData", 
        encryptedData,
        owner,
        dataName,
        big.NewInt(int64(ReleaseTime)),
        hash[:],
    )
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to pack transaction data: %v", err)})
        return
    }

    gasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to suggest gas price: %v", err)})
        return
    }

    gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
        From:     auth.From,
        To:       &contractAddress,
        Gas:      0,
        GasPrice: gasPrice,
        Value:    big.NewInt(0),
        Data:     input,
    })
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to estimate gas limit: %v", err)})
        return
    }

    gasLimit = uint64(float64(gasLimit) * 1.1)

    nonce, err := client.PendingNonceAt(context.Background(), auth.From)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to retrieve account nonce: %v", err)})
        return
    }

    tx := types.NewTransaction(nonce, contractAddress, big.NewInt(0), gasLimit, gasPrice, input)

    signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(43113)), privateKey)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to sign transaction: %v", err)})
        return
    }

    err = client.SendTransaction(context.Background(), signedTx)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to send transaction: %v", err)})
        return
    }

    receipt, err := bind.WaitMined(context.Background(), client, signedTx)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to get transaction receipt: %v", err)})
        return
    }

    c.JSON(200, gin.H{
        "message":         "Data published successfully",
        "transactionHash": receipt.TxHash.Hex(),
        "blockNumber":     receipt.BlockNumber.Uint64(),
    })
}

func getData(c *gin.Context) {
    dataName := c.Param("dataname")
    owner := c.Param("owner")

    input, err := contractABI.Pack("GetPublicData", dataName, owner)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to pack transaction data: %v", err)})
        return
    }

    msg := ethereum.CallMsg{
        To:   &contractAddress,
        Data: input,
    }

    output, err := client.CallContract(context.Background(), msg, nil)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to call contract: %v", err)})
        return
    }

    var result struct {
        EncryptedData []byte
        Hash          []byte
        Owner         string
        DataName      string
        ReleaseTime   *big.Int
        KeyReleased   bool
    }

    err = contractABI.UnpackIntoInterface(&result, "GetPublicData", output)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to unpack output: %v", err)})
        return
    }

    response := gin.H{
        "encryptedData": hexutil.Encode(result.EncryptedData),
        "hash":          hexutil.Encode(result.Hash),
        "owner":         result.Owner,
        "dataName":      result.DataName,
        "releaseTime":   result.ReleaseTime.String(),
        "keyReleased":   result.KeyReleased,
    }

    c.JSON(200, response)
}

func Web3Listener() {
    query := ethereum.FilterQuery{
        Addresses: []common.Address{contractAddress},
    }

    logs := make(chan types.Log)
    sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
    if err != nil {
        log.Fatalf("Failed to subscribe to logs: %v", err)
    }
    defer sub.Unsubscribe()

    for {
        select {
        case err := <-sub.Err():
            log.Printf("Subscription error: %v", err)
            time.Sleep(5 * time.Second)
            sub, err = client.SubscribeFilterLogs(context.Background(), query, logs)
            if err != nil {
                log.Printf("Failed to resubscribe to logs: %v", err)
            }
        case vLog := <-logs:
            handleLog(vLog)
        }
    }
}

func handleLog(vLog types.Log) {
    switch vLog.Topics[0].Hex() {
    case contractABI.Events["ReleaseEncryptedData"].ID.Hex():
        HandleReleaseEncryptedDataEvent(vLog)
    case contractABI.Events["KeyReleased"].ID.Hex():
        HandleKeyReleasedEvent(vLog)
    case contractABI.Events["KeyReleaseRequested"].ID.Hex():
        HandleKeyReleaseRequestedEvent(vLog)
    default:
        log.Println("Unknown event:", vLog.Topics[0].Hex())
    }
}

func HandleReleaseEncryptedDataEvent(vLog types.Log) {
    var event ReleaseEncryptedDataEvent

    err := contractABI.UnpackIntoInterface(&event, "ReleaseEncryptedData", vLog.Data)
    if err != nil {
        log.Printf("Failed to unpack ReleaseEncryptedData event: %v", err)
        return
    }

    fmt.Println("Encrypted Data: ", hexutil.Encode(event.EncryptedData))
    fmt.Println("Owner: ", event.Owner)
    fmt.Println("Data Name: ", event.DataName)
    fmt.Println("Release Time: ", event.ReleaseTime)
    fmt.Println("Hash: ", hexutil.Encode(event.Hash))
}


func HandleKeyReleasedEvent(vLog types.Log) {
    var event KeyReleasedEvent

    err := contractABI.UnpackIntoInterface(&event, "KeyReleased", vLog.Data)
    if err != nil {
        log.Printf("Failed to unpack KeyReleased event: %v", err)
        return
    }

    fmt.Println("Private Key: ", hexutil.Encode(event.PrivateKey))
    fmt.Println("Owner: ", event.Owner)
    fmt.Println("Data Name: ", event.DataName)
}


func HandleKeyReleaseRequestedEvent(vLog types.Log) {
    var event KeyReleaseRequestedEvent

    err := contractABI.UnpackIntoInterface(&event, "KeyReleaseRequested", vLog.Data)
    if err != nil {
        log.Printf("Failed to unpack KeyReleaseRequested event: %v", err)
        return
    }

    fmt.Println("Index: ", event.Index)
    fmt.Println("Owner: ", event.Owner)
    fmt.Println("Data Name: ", event.DataName)
}

func HandlePushPrivateKeyEvent(vLog types.Log) {
    var Event h.PushPrivateKeyEvent

    err := contractABI.UnpackIntoInterface(&Event, "PushPrivateKey", vLog.Data)
    if err != nil {
        log.Fatalf("Failed to unpack event: %v", err)
    }

    fmt.Println("Decryption Key: ", Event.DecryptionKey)
    fmt.Println("Owner: ", Event.Owner)
    fmt.Println("Data Name: ", Event.DataName)
    fmt.Println("Hash: ", Event.Hash)
}

func MustGetEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        log.Fatalf("Environment variable %s must be set", key)
    }
    return value
}
